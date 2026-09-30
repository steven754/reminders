package medkit

import "time"

const (
	PermissionView = "view"
	PermissionEdit = "edit"
)

// Medicine is owned by one application user. Medkit sharing never changes the
// owner of a medicine; it only grants another user scoped access to it.
type Medicine struct {
	ID              uint   `gorm:"primarykey" json:"id"`
	OwnerID         uint   `gorm:"not null;index:idx_medkit_medicines_owner" json:"owner_id"`
	Name            string `gorm:"size:120;not null" json:"name"`
	GenericName     string `gorm:"size:120" json:"generic_name"`
	Specification   string `gorm:"size:120" json:"specification"`
	Quantity        string `gorm:"size:40" json:"quantity"`
	Unit            string `gorm:"size:24" json:"unit"`
	ExpiryDate      string `gorm:"size:10;index" json:"expiry_date"`
	Notes           string `gorm:"type:text" json:"notes"`
	ReminderID      *uint  `gorm:"index" json:"reminder_id,omitempty"`
	ReminderEnabled bool   `gorm:"not null;default:true" json:"reminder_enabled"`
	ReminderDays    int    `gorm:"not null;default:7" json:"reminder_days"`
	ReminderTime    string `gorm:"size:5;not null;default:09:00" json:"reminder_time"`
	// ReminderChannels is kept in the generated reminder's channel rows. It is
	// ignored by GORM here and is populated for the Medkit API/UI only.
	ReminderChannels       []string          `gorm:"-" json:"reminder_channels"`
	ReminderChannelTargets map[string][]uint `gorm:"-" json:"reminder_channel_targets"`
	CreatedAt              time.Time         `json:"created_at"`
	UpdatedAt              time.Time         `json:"updated_at"`
}

func (Medicine) TableName() string { return "medkit_medicines" }

// Access grants a member user access to an owner's Medkit data.
// The relationship is intentionally one-way: the owner controls the grant.
type Access struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	OwnerID    uint      `gorm:"not null;uniqueIndex:idx_medkit_access_pair" json:"owner_id"`
	MemberID   uint      `gorm:"not null;uniqueIndex:idx_medkit_access_pair" json:"member_id"`
	Permission string    `gorm:"size:16;not null;default:view" json:"permission"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (Access) TableName() string { return "medkit_accesses" }

type AIConfig struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Enabled   bool      `gorm:"not null;default:false" json:"enabled"`
	BaseURL   string    `gorm:"size:300;not null" json:"base_url"`
	Model     string    `gorm:"size:120;not null" json:"model"`
	APIKey    string    `gorm:"type:text" json:"-"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AIConfig) TableName() string { return "medkit_ai_configs" }

type OwnerView struct {
	ID             uint   `json:"id"`
	Username       string `json:"username"`
	Permission     string `json:"permission"`
	CanEdit        bool   `json:"can_edit"`
	CanManageUsers bool   `json:"can_manage_users"`
}

type AccessView struct {
	Access
	MemberUsername string `json:"member_username"`
}
