package unit

import (
	"testing"

	"telegram_queue_bot/internal/validation"
	"telegram_queue_bot/pkg/errors"
)

func TestValidateSlotID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{
			name:    "valid slot ID",
			input:   "123",
			want:    123,
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			want:    0,
			wantErr: true,
		},
		{
			name:    "zero",
			input:   "0",
			want:    0,
			wantErr: true,
		},
		{
			name:    "negative number",
			input:   "-5",
			want:    0,
			wantErr: true,
		},
		{
			name:    "not a number",
			input:   "abc",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validation.ValidateSlotID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSlotID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ValidateSlotID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidatePhoneNumber(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid phone number",
			input:   "+1234567890",
			wantErr: false,
		},
		{
			name:    "valid international format",
			input:   "+79123456789",
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "no plus sign",
			input:   "1234567890",
			wantErr: true,
		},
		{
			name:    "starts with zero",
			input:   "+01234567890",
			wantErr: true,
		},
		{
			name:    "too short",
			input:   "+1", // Реально слишком короткий номер
			wantErr: true,
		},
		{
			name:    "contains letters",
			input:   "+123abc7890",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidatePhoneNumber(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePhoneNumber() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil {
				// Check that error is a BotError
				if !errors.IsBotError(err) {
					t.Errorf("Expected BotError, got %T", err)
				}
			}
		})
	}
}

func TestValidateDate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid future date",
			input:   "2025-12-31",
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid format",
			input:   "31-12-2025",
			wantErr: true,
		},
		{
			name:    "invalid date",
			input:   "2025-13-32",
			wantErr: true,
		},
		{
			name:    "past date",
			input:   "2020-01-01",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validation.ValidateDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid time",
			input:   "10:30",
			wantErr: false,
		},
		{
			name:    "midnight",
			input:   "00:00",
			wantErr: false,
		},
		{
			name:    "end of day",
			input:   "23:59",
			wantErr: false,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid format",
			input:   "10:30:00",
			wantErr: true,
		},
		{
			name:    "invalid hour",
			input:   "25:30",
			wantErr: true,
		},
		{
			name:    "invalid minute",
			input:   "10:60",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validation.ValidateTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTime() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateWorkingHours(t *testing.T) {
	tests := []struct {
		name      string
		time      string
		workStart string
		workEnd   string
		wantErr   bool
	}{
		{
			name:      "time within working hours",
			time:      "10:00",
			workStart: "09:00",
			workEnd:   "18:00",
			wantErr:   false,
		},
		{
			name:      "time at start of working hours",
			time:      "09:00",
			workStart: "09:00",
			workEnd:   "18:00",
			wantErr:   false,
		},
		{
			name:      "time before working hours",
			time:      "08:00",
			workStart: "09:00",
			workEnd:   "18:00",
			wantErr:   true,
		},
		{
			name:      "time at end of working hours",
			time:      "18:00",
			workStart: "09:00",
			workEnd:   "18:00",
			wantErr:   true,
		},
		{
			name:      "time after working hours",
			time:      "19:00",
			workStart: "09:00",
			workEnd:   "18:00",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateWorkingHours(tt.time, tt.workStart, tt.workEnd)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWorkingHours() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSlotDuration(t *testing.T) {
	tests := []struct {
		name      string
		startTime string
		endTime   string
		wantErr   bool
	}{
		{
			name:      "valid duration",
			startTime: "10:00",
			endTime:   "10:30",
			wantErr:   false,
		},
		{
			name:      "same start and end time",
			startTime: "10:00",
			endTime:   "10:00",
			wantErr:   true,
		},
		{
			name:      "end time before start time",
			startTime: "10:30",
			endTime:   "10:00",
			wantErr:   true,
		},
		{
			name:      "overnight duration",
			startTime: "23:00",
			endTime:   "01:00",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateSlotDuration(tt.startTime, tt.endTime)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSlotDuration() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateChatID(t *testing.T) {
	tests := []struct {
		name    string
		input   int64
		wantErr bool
	}{
		{
			name:    "positive chat ID",
			input:   12345,
			wantErr: false,
		},
		{
			name:    "negative chat ID (group)",
			input:   -12345,
			wantErr: false,
		},
		{
			name:    "zero chat ID",
			input:   0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateChatID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateChatID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateUserName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid name",
			input:   "John Doe",
			wantErr: false,
		},
		{
			name:    "single character name",
			input:   "J",
			wantErr: false,
		},
		{
			name:    "empty name",
			input:   "",
			wantErr: true,
		},
		{
			name:    "very long name",
			input:   string(make([]byte, 150)), // 150 characters
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validation.ValidateUserName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUserName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
