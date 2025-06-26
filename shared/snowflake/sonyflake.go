package snowflake

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/sony/sonyflake"
)

var sf *sonyflake.Sonyflake

// InitSonyFlake initializes the Sonyflake generator with default settings.
func InitSonyFlake() error {
	st := sonyflake.Settings{
		StartTime: time.Date(2022, time.October, 10, 0, 0, 0, 0, time.UTC),
	}

	// Initialize the global Sonyflake instance
	sf = sonyflake.NewSonyflake(st)
	if sf == nil {
		return errors.New("failed to initialize Sonyflake")
	}

	slog.Info("sonyflake initialized succesfully")
	return nil
}

// GenerateID generates a new Sonyflake ID.
func GenerateID() (uint64, error) {
	if sf == nil {
		return 0, errors.New("sonyflake is not initialized")
	}

	id, err := sf.NextID()
	if err != nil {
		return 0, fmt.Errorf("failed to generate Sonyflake ID: %w", err)
	}

	return id, nil
}
