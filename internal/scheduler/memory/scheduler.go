package memory

import (
	"context"
	"fmt"
	"sync"
	"telegram_queue_bot/internal/scheduler"
	"telegram_queue_bot/internal/storage/models"
	"time"
)

// MemoryScheduler реализует планировщик уведомлений в памяти
type MemoryScheduler struct {
	timers   map[int]*time.Timer
	mu       sync.RWMutex
	sender   scheduler.NotificationSender
	ctx      context.Context
	cancel   context.CancelFunc
	stopped  bool
	stopOnce sync.Once
}

// NewMemoryScheduler создает новый планировщик в памяти
func NewMemoryScheduler(sender scheduler.NotificationSender) *MemoryScheduler {
	ctx, cancel := context.WithCancel(context.Background())

	return &MemoryScheduler{
		timers: make(map[int]*time.Timer),
		sender: sender,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start запускает планировщик
func (s *MemoryScheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return fmt.Errorf("scheduler is stopped")
	}

	// Планировщик уже работает через контекст
	return nil
}

// Schedule планирует уведомление для слота
func (s *MemoryScheduler) Schedule(ctx context.Context, slot *models.Slot, notifyAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return fmt.Errorf("scheduler is stopped")
	}

	// Отменить существующий таймер если есть
	if timer, exists := s.timers[slot.ID]; exists {
		timer.Stop()
		delete(s.timers, slot.ID)
	}

	// Вычислить задержку до уведомления
	delay := time.Until(notifyAt)
	if delay <= 0 {
		// Если время уже прошло, отправить уведомление немедленно
		go s.handleNotification(slot)
		return nil
	}

	// Создать новый таймер
	timer := time.AfterFunc(delay, func() {
		s.handleNotification(slot)
	})

	s.timers[slot.ID] = timer
	return nil
}

// Cancel отменяет запланированное уведомление
func (s *MemoryScheduler) Cancel(ctx context.Context, slotID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if timer, exists := s.timers[slotID]; exists {
		timer.Stop()
		delete(s.timers, slotID)
	}

	return nil
}

// ReschedulePending перепланирует все ожидающие уведомления
func (s *MemoryScheduler) ReschedulePending(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Очистить все существующие таймеры
	for slotID, timer := range s.timers {
		timer.Stop()
		delete(s.timers, slotID)
	}

	// Здесь должна быть логика загрузки ожидающих уведомлений из БД
	// и их перепланирование, но для этого нужен доступ к storage

	return nil
}

// Stop останавливает планировщик
func (s *MemoryScheduler) Stop() error {
	var err error

	s.stopOnce.Do(func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		s.stopped = true

		// Остановить все таймеры
		for slotID, timer := range s.timers {
			timer.Stop()
			delete(s.timers, slotID)
		}

		// Отменить контекст
		s.cancel()
	})

	return err
}

// handleNotification обрабатывает отправку уведомления
func (s *MemoryScheduler) handleNotification(slot *models.Slot) {
	if s.stopped {
		return
	}

	// Удалить таймер из карты
	s.mu.Lock()
	delete(s.timers, slot.ID)
	s.mu.Unlock()

	// Отправить уведомление
	if err := s.sender.SendSlotReminder(s.ctx, slot); err != nil {
		// Здесь должно быть логирование ошибки
		// Пока что просто игнорируем
		return
	}
}

// GetActiveTimersCount возвращает количество активных таймеров (для отладки)
func (s *MemoryScheduler) GetActiveTimersCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.timers)
}
