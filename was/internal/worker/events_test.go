package worker

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEnvelopeValidation(t *testing.T) {
	valid := Event{ID: uuid.NewString(), AggregateID: uuid.NewString(), Type: "order.created", AggregateType: "order", AggregateVersion: 1, Version: 1, OccurredAt: time.Now()}
	if Validate(valid) != nil {
		t.Fatal("valid envelope rejected")
	}
	for _, change := range []func(*Event){func(e *Event) { e.Version = 2 }, func(e *Event) { e.ID = "bad" }, func(e *Event) { e.AggregateType = "payment" }, func(e *Event) { e.Type = "user-supplied-label" }} {
		e := valid
		change(&e)
		if !IsPermanent(Validate(e)) {
			t.Fatal("poison accepted")
		}
	}
}
