package models

import (
	"time"
)

// User represents an authenticated account holder
type User struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Email        string         `gorm:"uniqueIndex;size:255;not null" json:"email"`
	PasswordHash string         `gorm:"size:255;not null" json:"-"`
	FullName     string         `gorm:"size:255" json:"full_name"`
	Accounts     []EmailAccount `gorm:"foreignKey:UserID" json:"accounts,omitempty"`
	Expenses     []Expense      `gorm:"foreignKey:UserID" json:"expenses,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// EmailAccount stores a user's connected mailbox configuration (Gmail, Outlook, custom IMAP)
type EmailAccount struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	UserID            uint       `gorm:"index;not null" json:"user_id"`
	Provider          string     `gorm:"size:50;default:'Gmail'" json:"provider"` // "Gmail", "Outlook", "Yahoo", "iCloud", "Custom"
	EmailAddress      string     `gorm:"size:255;not null" json:"email_address"`
	IMAPServer        string     `gorm:"size:255;default:'imap.gmail.com:993'" json:"imap_server"`
	IMAPFolder        string     `gorm:"size:100;default:'INBOX'" json:"imap_folder"`
	EncryptedPassword string     `gorm:"type:text;not null" json:"-"` // AES-256-GCM encrypted App Password
	IsActive          bool       `gorm:"default:true" json:"is_active"`
	LastSyncAt        *time.Time `json:"last_sync_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
