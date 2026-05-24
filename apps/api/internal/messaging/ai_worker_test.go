package messaging

import (
	"testing"

	"github.com/projectx/api/internal/leads"
)

func TestAIWorker_AnalyzeSentiment(t *testing.T) {
	w := &AIWorker{}

	tests := []struct {
		name              string
		text              string
		expectedSentiment int
	}{
		{
			name:              "positive keyword - yes",
			text:              "yes, I am interested",
			expectedSentiment: 1,
		},
		{
			name:              "positive keyword - book it",
			text:              "book it please",
			expectedSentiment: 1,
		},
		{
			name:              "negative keyword - no thanks",
			text:              "no thanks, not for me",
			expectedSentiment: -1,
		},
		{
			name:              "neutral query",
			text:              "what times are available?",
			expectedSentiment: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sentiment, _, _ := w.analyzeSentiment(tt.text)
			if sentiment != tt.expectedSentiment {
				t.Errorf("analyzeSentiment(%q) = %v; want %v", tt.text, sentiment, tt.expectedSentiment)
			}
		})
	}
}

func TestAIWorker_DetectOptionChoice(t *testing.T) {
	w := &AIWorker{}

	tests := []struct {
		name           string
		body           string
		expectedStatus leads.LeadStatus
		expectedOk     bool
	}{
		{
			name:           "exact trial choice number",
			body:           "1",
			expectedStatus: leads.StatusTrialBooked,
			expectedOk:     true,
		},
		{
			name:           "exact membership choice number",
			body:           "2",
			expectedStatus: leads.StatusMember,
			expectedOk:     true,
		},
		{
			name:           "text - book a trial",
			body:           "I want to book a trial please",
			expectedStatus: leads.StatusTrialBooked,
			expectedOk:     true,
		},
		{
			name:           "text - trial only",
			body:           "trial option",
			expectedStatus: leads.StatusTrialBooked,
			expectedOk:     true,
		},
		{
			name:           "text - become a member",
			body:           "I would love to become a member",
			expectedStatus: leads.StatusMember,
			expectedOk:     true,
		},
		{
			name:           "text - membership only",
			body:           "membership",
			expectedStatus: leads.StatusMember,
			expectedOk:     true,
		},
		{
			name:           "no choice - general question",
			body:           "how much is it?",
			expectedStatus: "",
			expectedOk:     false,
		},
		{
			name:           "conflicting keywords",
			body:           "should I do a trial or become a member?",
			expectedStatus: "",
			expectedOk:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, ok := w.detectOptionChoice(tt.body)
			if ok != tt.expectedOk {
				t.Errorf("detectOptionChoice(%q) ok = %v; want %v", tt.body, ok, tt.expectedOk)
			}
			if status != tt.expectedStatus {
				t.Errorf("detectOptionChoice(%q) status = %v; want %v", tt.body, status, tt.expectedStatus)
			}
		})
	}
}
