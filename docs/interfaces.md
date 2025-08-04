# Интерфейсы между компонентами

## Storage Interfaces

### UserRepository
```go
type UserRepository interface {
    SaveUser(ctx context.Context, chatID int64, phone, firstName, lastName string) error
    IsUserRegistered(ctx context.Context, chatID int64) bool
    GetUserByID(ctx context.Context, chatID int64) (*User, error)
}
```

### SlotRepository  
```go
type SlotRepository interface {
    CreateSlot(ctx context.Context, slot *Slot) error
    GetAvailableSlots(ctx context.Context, date string) ([]*Slot, error)
    ReserveSlot(ctx context.Context, slotID int, chatID int64) error
    GetSlotByID(ctx context.Context, id int) (*Slot, error)
    GetUserTodaySlot(ctx context.Context, chatID int64) (*Slot, bool, error)
    MarkSlotNotified(ctx context.Context, slotID int) error
    GetPendingNotifications(ctx context.Context) ([]*Slot, error)
}
```

### Storage (Combined Interface)
```go
type Storage interface {
    UserRepository
    SlotRepository
    Close() error
}
```

## Scheduler Interfaces

### NotificationScheduler
```go
type NotificationScheduler interface {
    Schedule(ctx context.Context, slot *Slot) error
    Cancel(ctx context.Context, slotID int) error
    ReschedulePending(ctx context.Context) error
    Stop() error
}
```

### NotificationSender
```go
type NotificationSender interface {
    SendNotification(ctx context.Context, chatID int64, message string) error
}
```

## Bot Service Interface

### BotService
```go
type BotService interface {
    HandleStart(ctx context.Context, chatID int64) error
    HandleContact(ctx context.Context, chatID int64, contact *models.Contact) error
    HandleCallback(ctx context.Context, chatID int64, data string) error
    SendMessage(ctx context.Context, chatID int64, text string, keyboard *models.InlineKeyboardMarkup) error
}
```

## Configuration Interface

### Config
```go
type Config interface {
    GetTelegramToken() string
    GetWebhookURL() string
    GetDatabasePath() string
    GetWorkHours() (start, end string)
    GetSlotDuration() int
    GetScheduleDays() int
    Validate() error
}
```

## HTTP Server Interface

### Server
```go
type Server interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    RegisterHandlers(botService BotService)
}
```

## Logger Interface

### Logger
```go
type Logger interface {
    Info(msg string, fields ...Field)
    Error(msg string, err error, fields ...Field)
    Debug(msg string, fields ...Field)
    Warn(msg string, fields ...Field)
}
```

## Data Models

### User Model
```go
type User struct {
    ID        int64     `json:"id"`
    ChatID    int64     `json:"chat_id"`
    Phone     string    `json:"phone"`
    FirstName string    `json:"first_name"`
    LastName  string    `json:"last_name"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### Slot Model
```go
type Slot struct {
    ID         int       `json:"id"`
    Date       string    `json:"date"`
    StartTime  string    `json:"start_time"`
    EndTime    string    `json:"end_time"`
    UserChatID *int64    `json:"user_chat_id,omitempty"`
    Notified   bool      `json:"notified"`
    CreatedAt  time.Time `json:"created_at"`
    UpdatedAt  time.Time `json:"updated_at"`
}
```

## Error Types

### Custom Errors
```go
type BotError struct {
    Code    string
    Message string
    Err     error
}

var (
    ErrSlotAlreadyReserved = &BotError{
        Code:    "SLOT_RESERVED",
        Message: "slot is already reserved",
    }
    
    ErrUserNotRegistered = &BotError{
        Code:    "USER_NOT_REGISTERED", 
        Message: "user is not registered",
    }
    
    ErrInvalidSlotID = &BotError{
        Code:    "INVALID_SLOT_ID",
        Message: "invalid slot ID format",
    }
)
```

## Dependency Flow

```
main.go
  ├── Config
  ├── Storage (SQLite)
  ├── Scheduler (Memory)
  ├── BotService
  │   ├── Storage
  │   └── Scheduler
  └── HTTPServer
      └── BotService
```

## Interface Segregation Principle

Интерфейсы разделены по принципу ISP:

1. **UserRepository** - только операции с пользователями
2. **SlotRepository** - только операции со слотами  
3. **NotificationScheduler** - только планирование уведомлений
4. **NotificationSender** - только отправка уведомлений

Это позволяет:
- Легко тестировать каждый компонент отдельно
- Создавать mock'и для unit тестов
- Заменять реализации без изменения интерфейсов
- Следовать принципу единственной ответственности
