package policy

import (
	"testing"
	"time"
)

func TestCirclePolicyCanonicalString(t *testing.T) {
	p := CirclePolicy{
		CircleID:         "work",
		RegretThreshold:  40,
		NotifyThreshold:  60,
		UrgentThreshold:  80,
		DailyNotifyQuota: 5,
		DailyQueuedQuota: 20,
	}

	s1 := p.CanonicalString()
	s2 := p.CanonicalString()

	if s1 != s2 {
		t.Errorf("CanonicalString not deterministic: %q != %q", s1, s2)
	}

	expected := "circle:work|regret:40|notify:60|urgent:80|daily_notify:5|daily_queued:20"
	if s1 != expected {
		t.Errorf("CanonicalString = %q, want %q", s1, expected)
	}
}

func TestCirclePolicyWithHours(t *testing.T) {
	p := CirclePolicy{
		CircleID:         "work",
		RegretThreshold:  40,
		NotifyThreshold:  60,
		UrgentThreshold:  80,
		DailyNotifyQuota: 5,
		DailyQueuedQuota: 20,
		Hours: &HoursPolicy{
			AllowedWeekdays: 0b0011111,
			StartMinute:     480,
			EndMinute:       1080,
		},
	}

	s := p.CanonicalString()
	if s == "" {
		t.Error("CanonicalString should not be empty")
	}

	// Should contain hours section
	if !contains(s, "hours:") {
		t.Errorf("CanonicalString should contain hours: %s", s)
	}
}

func TestCirclePolicyValidation(t *testing.T) {
	tests := []struct {
		name    string
		policy  CirclePolicy
		wantErr bool
	}{
		{
			name: "valid policy",
			policy: CirclePolicy{
				CircleID:         "work",
				RegretThreshold:  40,
				NotifyThreshold:  60,
				UrgentThreshold:  80,
				DailyNotifyQuota: 5,
				DailyQueuedQuota: 20,
			},
			wantErr: false,
		},
		{
			name: "missing circle_id",
			policy: CirclePolicy{
				RegretThreshold:  40,
				NotifyThreshold:  60,
				UrgentThreshold:  80,
				DailyNotifyQuota: 5,
				DailyQueuedQuota: 20,
			},
			wantErr: true,
		},
		{
			name: "regret threshold out of range",
			policy: CirclePolicy{
				CircleID:         "work",
				RegretThreshold:  101,
				NotifyThreshold:  60,
				UrgentThreshold:  80,
				DailyNotifyQuota: 5,
				DailyQueuedQuota: 20,
			},
			wantErr: true,
		},
		{
			name: "non-monotonic thresholds (urgent < notify)",
			policy: CirclePolicy{
				CircleID:         "work",
				RegretThreshold:  40,
				NotifyThreshold:  80,
				UrgentThreshold:  60, // Less than notify
				DailyNotifyQuota: 5,
				DailyQueuedQuota: 20,
			},
			wantErr: true,
		},
		{
			name: "non-monotonic thresholds (notify < regret)",
			policy: CirclePolicy{
				CircleID:         "work",
				RegretThreshold:  60,
				NotifyThreshold:  40, // Less than regret
				UrgentThreshold:  80,
				DailyNotifyQuota: 5,
				DailyQueuedQuota: 20,
			},
			wantErr: true,
		},
		{
			name: "negative quota",
			policy: CirclePolicy{
				CircleID:         "work",
				RegretThreshold:  40,
				NotifyThreshold:  60,
				UrgentThreshold:  80,
				DailyNotifyQuota: -1,
				DailyQueuedQuota: 20,
			},
			wantErr: true,
		},
		{
			name: "invalid hours start",
			policy: CirclePolicy{
				CircleID:         "work",
				RegretThreshold:  40,
				NotifyThreshold:  60,
				UrgentThreshold:  80,
				DailyNotifyQuota: 5,
				DailyQueuedQuota: 20,
				Hours: &HoursPolicy{
					AllowedWeekdays: 0b0011111,
					StartMinute:     1500, // Invalid
					EndMinute:       1080,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTriggerPolicyValidation(t *testing.T) {
	tests := []struct {
		name    string
		policy  TriggerPolicy
		wantErr bool
	}{
		{
			name: "valid policy",
			policy: TriggerPolicy{
				Trigger:           "obligation.due_soon",
				MinLevel:          "",
				SuppressByDefault: false,
				RegretBias:        10,
			},
			wantErr: false,
		},
		{
			name: "missing trigger",
			policy: TriggerPolicy{
				Trigger:    "",
				RegretBias: 10,
			},
			wantErr: true,
		},
		{
			name: "bias too high",
			policy: TriggerPolicy{
				Trigger:    "test",
				RegretBias: 51,
			},
			wantErr: true,
		},
		{
			name: "bias too low",
			policy: TriggerPolicy{
				Trigger:    "test",
				RegretBias: -51,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPolicySetHashDeterminism(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	ps1 := DefaultPolicySet(now)
	ps2 := DefaultPolicySet(now)

	if ps1.Hash != ps2.Hash {
		t.Errorf("DefaultPolicySet hashes differ: %s != %s", ps1.Hash, ps2.Hash)
	}

	// Verify hash is stable across multiple calls
	hash1 := ps1.ComputeHash()
	hash2 := ps1.ComputeHash()
	if hash1 != hash2 {
		t.Errorf("ComputeHash not stable: %s != %s", hash1, hash2)
	}
}

func TestPolicySetMapOrdering(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Create two policy sets with circles added in different orders
	ps1 := PolicySet{
		Version:    1,
		CapturedAt: now,
		Circles:    make(map[string]CirclePolicy),
		Triggers:   make(map[string]TriggerPolicy),
	}
	ps1.Circles["work"] = MinimalCirclePolicy("work")
	ps1.Circles["family"] = MinimalCirclePolicy("family")
	ps1.Circles["personal"] = MinimalCirclePolicy("personal")

	ps2 := PolicySet{
		Version:    1,
		CapturedAt: now,
		Circles:    make(map[string]CirclePolicy),
		Triggers:   make(map[string]TriggerPolicy),
	}
	// Add in different order
	ps2.Circles["personal"] = MinimalCirclePolicy("personal")
	ps2.Circles["work"] = MinimalCirclePolicy("work")
	ps2.Circles["family"] = MinimalCirclePolicy("family")

	ps1.ComputeHash()
	ps2.ComputeHash()

	if ps1.Hash != ps2.Hash {
		t.Errorf("Hash differs due to map ordering: %s != %s", ps1.Hash, ps2.Hash)
	}
}

func TestHoursPolicyIsAllowed(t *testing.T) {
	// Mon-Fri, 8am-6pm
	h := HoursPolicy{
		AllowedWeekdays: 0b0011111, // Mon=0, Tue=1, Wed=2, Thu=3, Fri=4
		StartMinute:     8 * 60,    // 8:00 AM
		EndMinute:       18 * 60,   // 6:00 PM
	}

	tests := []struct {
		name string
		time time.Time
		want bool
	}{
		{
			name: "Monday 10am - allowed",
			time: time.Date(2025, 1, 6, 10, 0, 0, 0, time.UTC), // Monday
			want: true,
		},
		{
			name: "Monday 7am - before hours",
			time: time.Date(2025, 1, 6, 7, 0, 0, 0, time.UTC), // Monday
			want: false,
		},
		{
			name: "Monday 7pm - after hours",
			time: time.Date(2025, 1, 6, 19, 0, 0, 0, time.UTC), // Monday
			want: false,
		},
		{
			name: "Saturday 10am - weekend",
			time: time.Date(2025, 1, 4, 10, 0, 0, 0, time.UTC), // Saturday
			want: false,
		},
		{
			name: "Sunday 10am - weekend",
			time: time.Date(2025, 1, 5, 10, 0, 0, 0, time.UTC), // Sunday
			want: false,
		},
		{
			name: "Friday 6pm - edge",
			time: time.Date(2025, 1, 10, 18, 0, 0, 0, time.UTC), // Friday
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.IsAllowed(tt.time)
			if got != tt.want {
				t.Errorf("IsAllowed(%v) = %v, want %v", tt.time, got, tt.want)
			}
		})
	}
}

func TestDefaultPolicySetValid(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	ps := DefaultPolicySet(now)

	if err := ps.Validate(); err != nil {
		t.Errorf("DefaultPolicySet should be valid: %v", err)
	}

	// Should have expected circles
	expectedCircles := []string{"work", "family", "personal", "finance"}
	for _, c := range expectedCircles {
		if ps.GetCircle(c) == nil {
			t.Errorf("DefaultPolicySet missing circle: %s", c)
		}
	}

	// Should have expected triggers
	expectedTriggers := []string{"obligation.due_soon", "obligation.overdue", "balance.low", "newsletter", "marketing"}
	for _, tr := range expectedTriggers {
		if ps.GetTrigger(tr) == nil {
			t.Errorf("DefaultPolicySet missing trigger: %s", tr)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
