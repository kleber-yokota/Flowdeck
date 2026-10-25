package buy

import (
	"context"
	"testing"
)

func TestChargeStepCharges(t *testing.T) {
	step := ChargeStep("order-42")
	result, err := step(context.Background())
	if err != nil {
		t.Fatalf("step: %v", err)
	}
	if result != "charged: order-42" {
		t.Fatalf("result: got %q", result)
	}
}
