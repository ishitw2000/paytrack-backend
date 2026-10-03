package scheduler

import (
	"context"
	"log"

	"github.com/paytrack/backend/internal/config"
	"github.com/paytrack/backend/internal/service"
	"github.com/robfig/cron/v3"
)

type CronScheduler struct {
	cron        *cron.Cron
	cfg         *config.Config
	syncService *service.SyncService
}

func NewCronScheduler(cfg *config.Config, syncService *service.SyncService) *CronScheduler {
	return &CronScheduler{
		cron:        cron.New(),
		cfg:         cfg,
		syncService: syncService,
	}
}

// Start registers the recurring sync job and starts the scheduler in background
func (cs *CronScheduler) Start() error {
	if !cs.cfg.CronEnabled {
		log.Println("[SCHEDULER] Automated email sync is disabled in config (CRON_ENABLED=false)")
		return nil
	}

	schedule := cs.cfg.CronSchedule
	if schedule == "" {
		schedule = "0 0,6,12,18 * * *" // 4 times a day: 00:00, 06:00, 12:00, 18:00
	}

	_, err := cs.cron.AddFunc(schedule, func() {
		log.Println("[SCHEDULER] Triggering scheduled multi-user email sync...")
		ctx := context.Background()
		syncErr := cs.syncService.SyncAllActiveAccounts(ctx, "CRON")
		if syncErr != nil {
			log.Printf("[SCHEDULER] Scheduled multi-user sync error: %v\n", syncErr)
		}
	})

	if err != nil {
		return err
	}

	cs.cron.Start()
	log.Printf("[SCHEDULER] Cron scheduler started with schedule: '%s' (4x daily sync active across all users)\n", schedule)
	return nil
}

// Stop gracefully stops the cron runner
func (cs *CronScheduler) Stop() {
	if cs.cron != nil {
		cs.cron.Stop()
		log.Println("[SCHEDULER] Cron scheduler stopped.")
	}
}
