package unit

import (
	"testing"
	"time"

	"telegram_queue_bot/internal/storage/models"
	"telegram_queue_bot/tests/testutils"
)

func TestUserRepository_SaveUser(t *testing.T) {
	storage := testutils.SetupTestDB(t)
	ctx := testutils.TestContext()

	// Test data
	chatID := int64(12345)
	phone := "+1234567890"
	firstName := "John"
	lastName := "Doe"

	// Save user
	err := storage.SaveUser(ctx, chatID, phone, firstName, lastName)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Check if user is registered
	registered, err := storage.IsUserRegistered(ctx, chatID)
	if err != nil {
		t.Fatalf("expected no error when checking registration, got %v", err)
	}
	if !registered {
		t.Fatal("expected user to be registered")
	}

	// Get user by ID
	user, err := storage.GetUserByID(ctx, chatID)
	if err != nil {
		t.Fatalf("expected no error when getting user, got %v", err)
	}

	if user == nil {
		t.Fatal("expected user to be found")
	}

	if user.ChatID != chatID {
		t.Errorf("expected chat_id %d, got %d", chatID, user.ChatID)
	}

	if user.Phone != phone {
		t.Errorf("expected phone %s, got %s", phone, user.Phone)
	}

	if user.FirstName != firstName {
		t.Errorf("expected first_name %s, got %s", firstName, user.FirstName)
	}

	if user.LastName != lastName {
		t.Errorf("expected last_name %s, got %s", lastName, user.LastName)
	}
}

func TestUserRepository_DuplicateUser(t *testing.T) {
	storage := testutils.SetupTestDB(t)
	ctx := testutils.TestContext()

	chatID := int64(12345)
	phone := "+1234567890"
	firstName := "John"
	lastName := "Doe"

	// Save user first time
	err := storage.SaveUser(ctx, chatID, phone, firstName, lastName)
	if err != nil {
		t.Fatalf("expected no error on first save, got %v", err)
	}

	// Save the same user again (should succeed because of INSERT OR REPLACE)
	err = storage.SaveUser(ctx, chatID, phone, "Updated"+firstName, lastName)
	if err != nil {
		t.Fatalf("expected no error on second save (INSERT OR REPLACE), got %v", err)
	}

	// Verify the user was updated
	user, err := storage.GetUserByID(ctx, chatID)
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}

	expected := "Updated" + firstName
	testutils.AssertEqual(t, expected, user.FirstName, "User should be updated")
}

func TestSlotRepository_CreateSlot(t *testing.T) {
	storage := testutils.SetupTestDB(t)
	ctx := testutils.TestContext()

	slot := &models.Slot{
		Date:      "2025-08-05",
		StartTime: "10:00",
		EndTime:   "10:30",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := storage.CreateSlot(ctx, slot)
	if err != nil {
		t.Fatalf("expected no error when creating slot, got %v", err)
	}

	if slot.ID == 0 {
		t.Fatal("expected slot ID to be set after creation")
	}
}

func TestSlotRepository_GetAvailableSlots(t *testing.T) {
	storage := testutils.SetupTestDB(t)
	ctx := testutils.TestContext()

	date := "2025-08-05"

	// Создаем тестового пользователя для зарезервированного слота
	user := testutils.CreateTestUser(t, storage, 123)

	// Create some test slots
	slots := []*models.Slot{
		{
			Date:      date,
			StartTime: "10:00",
			EndTime:   "10:30",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			Date:      date,
			StartTime: "11:00",
			EndTime:   "11:30",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			Date:       date,
			StartTime:  "12:00",
			EndTime:    "12:30",
			UserChatID: &user.ChatID, // Используем существующего пользователя
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
	}

	for _, slot := range slots {
		err := storage.CreateSlot(ctx, slot)
		if err != nil {
			t.Fatalf("failed to create test slot: %v", err)
		}
	}

	// Get available slots
	availableSlots, err := storage.GetAvailableSlots(ctx, date)
	if err != nil {
		t.Fatalf("expected no error when getting available slots, got %v", err)
	}

	// Should return only unreserved slots
	expectedCount := 2
	if len(availableSlots) != expectedCount {
		t.Errorf("expected %d available slots, got %d", expectedCount, len(availableSlots))
	}

	// Check that reserved slot is not in the list
	for _, slot := range availableSlots {
		if slot.UserChatID != nil {
			t.Error("found reserved slot in available slots list")
		}
	}
}

func TestSlotRepository_ReserveSlot(t *testing.T) {
	storage := testutils.SetupTestDB(t)
	ctx := testutils.TestContext()

	// Create a user first
	chatID := int64(12345)
	err := storage.SaveUser(ctx, chatID, "+1234567890", "John", "Doe")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create a slot
	slot := &models.Slot{
		Date:      "2025-08-05",
		StartTime: "10:00",
		EndTime:   "10:30",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = storage.CreateSlot(ctx, slot)
	if err != nil {
		t.Fatalf("failed to create test slot: %v", err)
	}

	// Reserve the slot
	err = storage.ReserveSlot(ctx, slot.ID, chatID)
	if err != nil {
		t.Fatalf("expected no error when reserving slot, got %v", err)
	}

	// Verify the slot is reserved
	reservedSlot, err := storage.GetSlotByID(ctx, slot.ID)
	if err != nil {
		t.Fatalf("failed to get slot by ID: %v", err)
	}

	if reservedSlot.UserChatID == nil {
		t.Fatal("expected slot to be reserved")
	}

	if *reservedSlot.UserChatID != chatID {
		t.Errorf("expected slot to be reserved by user %d, got %d", chatID, *reservedSlot.UserChatID)
	}
}

func TestSlotRepository_ReserveAlreadyReservedSlot(t *testing.T) {
	storage := testutils.SetupTestDB(t)
	ctx := testutils.TestContext()

	// Create users
	chatID1 := int64(12345)
	chatID2 := int64(67890)

	err := storage.SaveUser(ctx, chatID1, "+1234567890", "John", "Doe")
	if err != nil {
		t.Fatalf("failed to create test user 1: %v", err)
	}

	err = storage.SaveUser(ctx, chatID2, "+0987654321", "Jane", "Smith")
	if err != nil {
		t.Fatalf("failed to create test user 2: %v", err)
	}

	// Create a slot
	slot := &models.Slot{
		Date:      "2025-08-05",
		StartTime: "10:00",
		EndTime:   "10:30",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = storage.CreateSlot(ctx, slot)
	if err != nil {
		t.Fatalf("failed to create test slot: %v", err)
	}

	// Reserve the slot with first user
	err = storage.ReserveSlot(ctx, slot.ID, chatID1)
	if err != nil {
		t.Fatalf("expected no error when reserving slot with first user, got %v", err)
	}

	// Try to reserve the same slot with second user
	err = storage.ReserveSlot(ctx, slot.ID, chatID2)
	if err == nil {
		t.Fatal("expected error when trying to reserve already reserved slot")
	}
}

func TestSlotRepository_CancelSlot(t *testing.T) {
	storage := testutils.SetupTestDB(t)
	ctx := testutils.TestContext()

	// Create a user
	chatID := int64(12345)
	err := storage.SaveUser(ctx, chatID, "+1234567890", "John", "Doe")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Create and reserve a slot
	slot := &models.Slot{
		Date:      "2025-08-05",
		StartTime: "10:00",
		EndTime:   "10:30",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = storage.CreateSlot(ctx, slot)
	if err != nil {
		t.Fatalf("failed to create test slot: %v", err)
	}

	err = storage.ReserveSlot(ctx, slot.ID, chatID)
	if err != nil {
		t.Fatalf("failed to reserve test slot: %v", err)
	}

	// Cancel the slot
	err = storage.CancelSlot(ctx, slot.ID, chatID)
	if err != nil {
		t.Fatalf("expected no error when cancelling slot, got %v", err)
	}

	// Verify the slot is no longer reserved
	cancelledSlot, err := storage.GetSlotByID(ctx, slot.ID)
	if err != nil {
		t.Fatalf("failed to get slot by ID after cancellation: %v", err)
	}

	if cancelledSlot.UserChatID != nil {
		t.Error("expected slot to be unreserved after cancellation")
	}
}
