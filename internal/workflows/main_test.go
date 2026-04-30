package workflows

import (
	"testing"
)

func TestMain(m *testing.M) {
	StepTypeValidator = func(typ string) bool {
		switch typ {
		case "transform", "shell", "wait":
			return true
		default:
			return false
		}
	}
	m.Run()
}
