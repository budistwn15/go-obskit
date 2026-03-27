package gormx

import "testing"

func TestFingerprintSQL_EquivalentQueriesMatch(t *testing.T) {
	q1 := "SELECT * FROM users WHERE id=1 AND status='active'"
	q2 := "select * from users where id=999 and status='inactive'"
	if fingerprintSQL(q1) == "" || fingerprintSQL(q2) == "" {
		t.Fatalf("fingerprint should not be empty")
	}
	if fingerprintSQL(q1) != fingerprintSQL(q2) {
		t.Fatalf("equivalent query shapes should have same fingerprint")
	}
}

func TestFingerprintSQL_DifferentShapeDiffers(t *testing.T) {
	q1 := "SELECT * FROM users WHERE id=1"
	q2 := "UPDATE users SET status='active' WHERE id=1"
	if fingerprintSQL(q1) == fingerprintSQL(q2) {
		t.Fatalf("different query shape should produce different fingerprint")
	}
}
