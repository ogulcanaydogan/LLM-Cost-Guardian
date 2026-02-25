package model_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/ogulcanaydogan/LLM-Cost-Guardian/pkg/model"
)

func TestPeriodBounds_Daily(t *testing.T) {
	start, end := model.PeriodBounds(model.PeriodDaily)
	assert.False(t, start.IsZero())
	assert.False(t, end.IsZero())
	assert.Equal(t, 24*time.Hour, end.Sub(start))
	assert.Equal(t, 0, start.Hour())
	assert.Equal(t, 0, start.Minute())
}

func TestPeriodBounds_Weekly(t *testing.T) {
	start, end := model.PeriodBounds(model.PeriodWeekly)
	assert.False(t, start.IsZero())
	assert.Equal(t, 7*24*time.Hour, end.Sub(start))
}

func TestPeriodBounds_Monthly(t *testing.T) {
	start, end := model.PeriodBounds(model.PeriodMonthly)
	assert.False(t, start.IsZero())
	assert.Equal(t, 1, start.Day())
	assert.True(t, end.After(start))
}

func TestPeriodBounds_Default(t *testing.T) {
	start, end := model.PeriodBounds("unknown")
	assert.False(t, start.IsZero())
	assert.Equal(t, 24*time.Hour, end.Sub(start))
}
