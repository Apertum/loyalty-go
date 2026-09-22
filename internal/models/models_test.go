package models

import "testing"

func TestOrderStatusConstants(t *testing.T) {
	if OrderStatusNew != "NEW" {
		t.Errorf("expected NEW, got %s", OrderStatusNew)
	}
	if OrderStatusProcessing != "PROCESSING" {
		t.Errorf("expected PROCESSING, got %s", OrderStatusProcessing)
	}
	if OrderStatusInvalid != "INVALID" {
		t.Errorf("expected INVALID, got %s", OrderStatusInvalid)
	}
	if OrderStatusProcessed != "PROCESSED" {
		t.Errorf("expected PROCESSED, got %s", OrderStatusProcessed)
	}
}

func TestTransactionTypeConstants(t *testing.T) {
	if TransactionTypeEarned != "earned" {
		t.Errorf("expected earned, got %s", TransactionTypeEarned)
	}
	if TransactionTypeRedeemed != "redeemed" {
		t.Errorf("expected redeemed, got %s", TransactionTypeRedeemed)
	}
}

func TestBalanceResponseZero(t *testing.T) {
	br := BalanceResponse{}
	if br.Current != 0 {
		t.Errorf("expected current 0, got %f", br.Current)
	}
	if br.Withdrawn != 0 {
		t.Errorf("expected withdrawn 0, got %d", br.Withdrawn)
	}
}

func TestOrderCreation(t *testing.T) {
	o := Order{}
	if o.Status != "" {
		t.Errorf("expected empty status, got %s", o.Status)
	}
	if o.Accrual != nil {
		t.Errorf("expected nil accrual, got %v", o.Accrual)
	}
}

func TestTransactionCreation(t *testing.T) {
	txn := Transaction{}
	if txn.Type != "" {
		t.Errorf("expected empty type, got %s", txn.Type)
	}
}

func TestUserCreation(t *testing.T) {
	u := User{}
	if u.Login != "" {
		t.Errorf("expected empty login, got %s", u.Login)
	}
}
