package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/paytrack/backend/internal/config"
	"github.com/paytrack/backend/internal/crypto"
	"github.com/paytrack/backend/internal/email"
	"github.com/paytrack/backend/internal/llm"
	"github.com/paytrack/backend/internal/models"
	"gorm.io/gorm"
)

type SyncService struct {
	cfg        *config.Config
	db         *gorm.DB
	imapClient *email.IMAPClient
	extractor  *llm.PaymentExtractor
	mu         sync.Mutex
	isSyncing  bool
	lastSyncAt *time.Time
}

func NewSyncService(cfg *config.Config, db *gorm.DB) *SyncService {
	return &SyncService{
		cfg:        cfg,
		db:         db,
		imapClient: email.NewIMAPClient(cfg),
		extractor:  llm.NewPaymentExtractor(cfg),
	}
}

func (s *SyncService) IsSyncing() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isSyncing
}

func (s *SyncService) LastSyncedAt() *time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastSyncAt
}

// SyncUserAccount executes email sync for a specific connected EmailAccount of a user
func (s *SyncService) SyncUserAccount(ctx context.Context, account *models.EmailAccount, triggerType string) (*models.SyncLog, error) {
	plainPassword, err := crypto.Decrypt(account.EncryptedPassword, s.cfg.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt account credentials: %w", err)
	}

	startTime := time.Now()
	syncLog := models.SyncLog{
		UserID:      account.UserID,
		AccountID:   account.ID,
		StartedAt:   startTime,
		Status:      "RUNNING",
		TriggerType: triggerType,
	}
	s.db.Create(&syncLog)

	emails, err := s.imapClient.FetchEmailsForCredentials(
		account.IMAPServer,
		account.EmailAddress,
		plainPassword,
		account.IMAPFolder,
		s.cfg.FetchBatchSize,
		s.cfg.LookbackDays,
	)
	if err != nil {
		syncLog.Status = "FAILED"
		syncLog.CompletedAt = time.Now()
		syncLog.ErrorMessage = err.Error()
		s.db.Save(&syncLog)
		return &syncLog, err
	}

	syncLog.EmailsProcessed = len(emails)
	paymentsFound := 0
	newSaved := 0

	for _, em := range emails {
		if em.MessageID != "" {
			var existing models.Expense
			if err := s.db.Where("user_id = ? AND message_id = ?", account.UserID, em.MessageID).First(&existing).Error; err == nil {
				continue
			}
		}

		extracted := s.extractor.ExtractPaymentDetails(ctx, em.Subject, em.Sender, em.Body)
		if !extracted.IsPayment || extracted.Amount <= 0 {
			continue
		}

		paymentsFound++

		txDate := em.Date
		if extracted.DateStr != "" {
			if parsed, err := time.Parse("2006-01-02", extracted.DateStr); err == nil {
				txDate = parsed
			}
		}

		// Deduplication check per user
		var dup models.Expense
		if err := s.db.Where("user_id = ? AND amount = ? AND merchant = ? AND date BETWEEN ? AND ?",
			account.UserID,
			extracted.Amount,
			extracted.Merchant,
			txDate.Add(-12*time.Hour),
			txDate.Add(12*time.Hour),
		).First(&dup).Error; err == nil {
			continue
		}

		currency := extracted.Currency
		if currency == "" {
			currency = "USD"
		}

		var msgID *string
		if em.MessageID != "" {
			msgID = &em.MessageID
		}

		expense := models.Expense{
			UserID:          account.UserID,
			AccountID:       account.ID,
			EmailUID:        em.UID,
			MessageID:       msgID,
			Sender:          em.Sender,
			RawSubject:      em.Subject,
			Date:            txDate,
			Amount:          extracted.Amount,
			Currency:        currency,
			Merchant:        extracted.Merchant,
			Category:        extracted.Category,
			PaymentMethod:   extracted.PaymentMethod,
			TransactionType: extracted.TransactionType,
			ConfidenceScore: extracted.ConfidenceScore,
			Summary:         extracted.Summary,
			IsManual:        false,
		}

		if err := s.db.Create(&expense).Error; err == nil {
			newSaved++
		}
	}

	now := time.Now()
	account.LastSyncAt = &now
	s.db.Save(account)

	syncLog.PaymentsFound = paymentsFound
	syncLog.NewExpensesSaved = newSaved
	syncLog.CompletedAt = now
	syncLog.Status = "SUCCESS"
	s.db.Save(&syncLog)

	return &syncLog, nil
}

// SyncAllActiveAccounts uses a concurrent worker pool to sync all users' connected mailboxes in parallel
func (s *SyncService) SyncAllActiveAccounts(ctx context.Context, triggerType string) error {
	s.mu.Lock()
	if s.isSyncing {
		s.mu.Unlock()
		return fmt.Errorf("sync is already in progress")
	}
	s.isSyncing = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.isSyncing = false
		now := time.Now()
		s.lastSyncAt = &now
		s.mu.Unlock()
	}()

	var accounts []models.EmailAccount
	if err := s.db.Where("is_active = ?", true).Find(&accounts).Error; err != nil {
		return err
	}

	numAccounts := len(accounts)
	log.Printf("[SYNC-WORKER-POOL] Starting parallel sync for %d active mailboxes across users...\n", numAccounts)

	if numAccounts > 0 {
		numWorkers := s.cfg.SyncWorkers
		if numWorkers <= 0 {
			numWorkers = 5
		}
		if numWorkers > numAccounts {
			numWorkers = numAccounts
		}

		accountChan := make(chan models.EmailAccount, numAccounts)
		var wg sync.WaitGroup

		for w := 1; w <= numWorkers; w++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for acc := range accountChan {
					log.Printf("[WORKER-%d] Syncing mailbox ID=%d for User=%d (%s)\n", workerID, acc.ID, acc.UserID, acc.EmailAddress)
					_, err := s.SyncUserAccount(ctx, &acc, triggerType)
					if err != nil {
						log.Printf("[WORKER-%d] Error syncing mailbox %s: %v\n", workerID, acc.EmailAddress, err)
					}
				}
			}(w)
		}

		for _, acc := range accounts {
			accountChan <- acc
		}
		close(accountChan)

		wg.Wait()
		log.Printf("[SYNC-WORKER-POOL] Completed parallel sync for all %d mailboxes.\n", numAccounts)
	}

	// Standalone fallback
	if s.cfg.IMAPUsername != "" && s.cfg.IMAPPassword != "" {
		log.Println("[SYNC-ALL] Also running standalone .env fallback sync...")
		_, _ = s.PerformSync(ctx, triggerType)
	}

	return nil
}

// PerformSync runs standalone sync using .env credentials (fallback)
func (s *SyncService) PerformSync(ctx context.Context, triggerType string) (*models.SyncLog, error) {
	startTime := time.Now()
	syncLog := models.SyncLog{
		UserID:      1,
		StartedAt:   startTime,
		Status:      "RUNNING",
		TriggerType: triggerType,
	}
	s.db.Create(&syncLog)

	emails, err := s.imapClient.FetchRecentEmails()
	if err != nil {
		syncLog.Status = "FAILED"
		syncLog.CompletedAt = time.Now()
		syncLog.ErrorMessage = err.Error()
		s.db.Save(&syncLog)
		return &syncLog, err
	}

	syncLog.EmailsProcessed = len(emails)
	paymentsFound := 0
	newSaved := 0

	for _, em := range emails {
		if em.MessageID != "" {
			var existing models.Expense
			if err := s.db.Where("user_id = ? AND message_id = ?", 1, em.MessageID).First(&existing).Error; err == nil {
				continue
			}
		}

		extracted := s.extractor.ExtractPaymentDetails(ctx, em.Subject, em.Sender, em.Body)
		if !extracted.IsPayment || extracted.Amount <= 0 {
			continue
		}

		paymentsFound++
		txDate := em.Date
		if extracted.DateStr != "" {
			if parsed, err := time.Parse("2006-01-02", extracted.DateStr); err == nil {
				txDate = parsed
			}
		}

		var dup models.Expense
		if err := s.db.Where("user_id = ? AND amount = ? AND merchant = ? AND date BETWEEN ? AND ?",
			1, extracted.Amount, extracted.Merchant, txDate.Add(-12*time.Hour), txDate.Add(12*time.Hour),
		).First(&dup).Error; err == nil {
			continue
		}

		currency := extracted.Currency
		if currency == "" {
			currency = "USD"
		}

		var msgID *string
		if em.MessageID != "" {
			msgID = &em.MessageID
		}

		expense := models.Expense{
			UserID:          1,
			EmailUID:        em.UID,
			MessageID:       msgID,
			Sender:          em.Sender,
			RawSubject:      em.Subject,
			Date:            txDate,
			Amount:          extracted.Amount,
			Currency:        currency,
			Merchant:        extracted.Merchant,
			Category:        extracted.Category,
			PaymentMethod:   extracted.PaymentMethod,
			TransactionType: extracted.TransactionType,
			ConfidenceScore: extracted.ConfidenceScore,
			Summary:         extracted.Summary,
			IsManual:        false,
		}

		if err := s.db.Create(&expense).Error; err == nil {
			newSaved++
		}
	}

	syncLog.PaymentsFound = paymentsFound
	syncLog.NewExpensesSaved = newSaved
	syncLog.CompletedAt = time.Now()
	syncLog.Status = "SUCCESS"
	s.db.Save(&syncLog)

	return &syncLog, nil
}
