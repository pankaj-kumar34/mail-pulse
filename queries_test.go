package main

import "testing"

func TestLooksRandom(t *testing.T) {
	garbage := []string{"xk7fj29dqmz", "q9w8e7r5t4y", "ab12cd34ef56", "jsmith660127", "user20240x9z"}
	for _, s := range garbage {
		if !looksRandom(s) {
			t.Errorf("looksRandom(%q) = false, want true", s)
		}
	}
	real := []string{"john.smith", "sarah.chen", "ops-team", "notifications", "pankaj.kumar", "support.team2", "jdoe"}
	for _, s := range real {
		if looksRandom(s) {
			t.Errorf("looksRandom(%q) = true, want false", s)
		}
	}
}

func TestAddrFlags(t *testing.T) {
	if f := addrFlags("dps-uat@openpages.local"); len(f) == 0 || f[0] != "reserved-tld" {
		t.Errorf("addrFlags reserved-tld = %v", f)
	}
	if f := addrFlags("xk7fj29dqmz@mail.com"); len(f) == 0 || f[0] != "random-localpart" {
		t.Errorf("addrFlags random-localpart = %v", f)
	}
	if f := addrFlags("john.smith@ibm.com"); len(f) != 0 {
		t.Errorf("addrFlags clean addr = %v, want none", f)
	}
	if f := addrFlags("no-at-sign"); len(f) != 0 {
		t.Errorf("addrFlags no domain = %v, want none", f)
	}
}
