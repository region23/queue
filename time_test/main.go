package main

import (
	"fmt"
	"time"
)

func main() {
	now := time.Now()
	today := now.Format("2006-01-02")
	currentTime := now.Format("15:04")

	fmt.Printf("Сегодня: %s\n", today)
	fmt.Printf("Текущее время: %s\n", currentTime)

	// Тестовые слоты
	testSlots := []string{"08:00", "09:30", "11:00", "14:00", "16:30", "18:00"}

	fmt.Println("\nДоступные слоты на сегодня (после текущего времени):")
	for _, slot := range testSlots {
		if slot > currentTime {
			fmt.Printf("✓ %s - доступен\n", slot)
		} else {
			fmt.Printf("✗ %s - прошел\n", slot)
		}
	}
}
