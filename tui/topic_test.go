package tui

import (
	"strings"
	"testing"

	"github.com/hrhrprasath/dhi/internal/topic"

	"charm.land/bubbles/v2/viewport"
)

func TestTopicPanel_DefaultFields(t *testing.T) {
	tp := &topicPanel{
		view: viewport.New(viewport.WithWidth(28), viewport.WithHeight(10)),
	}
	if tp.popupOpen {
		t.Error("popupOpen should be false by default")
	}
	if tp.selIdx != 0 {
		t.Errorf("selIdx should be 0, got %d", tp.selIdx)
	}
}

func TestTopicPanel_RenderRecentEmpty(t *testing.T) {
	tp := &topicPanel{
		view: viewport.New(viewport.WithWidth(28), viewport.WithHeight(10)),
	}
	out := tp.renderRecent(nil, nil, false, 30)
	if !strings.Contains(out, "RECENT TOPICS") {
		t.Errorf("renderRecent() should contain 'RECENT TOPICS', got: %s", out)
	}
	if !strings.Contains(out, "No topics") {
		t.Errorf("renderRecent() should contain 'No topics', got: %s", out)
	}
}

func TestTopicPanel_RenderRecentWithTopics(t *testing.T) {
	tp := &topicPanel{
		recentIdx: 0,
		view:      viewport.New(viewport.WithWidth(28), viewport.WithHeight(10)),
	}
	all := []topic.Topic{
		{ID: "1", Title: "Golang discussion"},
		{ID: "2", Title: "AI news"},
	}
	out := tp.renderRecent(all, nil, true, 30)
	if !strings.Contains(out, "Golang") {
		t.Errorf("renderRecent() should contain 'Golang', got: %s", out)
	}
	if !strings.Contains(out, "AI news") {
		t.Errorf("renderRecent() should contain 'AI news', got: %s", out)
	}
}

func TestTopicPanel_RenderRecentCurrentTopic(t *testing.T) {
	tp := &topicPanel{
		recentIdx: 1,
		view:      viewport.New(viewport.WithWidth(28), viewport.WithHeight(10)),
	}
	cur := &topic.Topic{ID: "2", Title: "Current topic"}
	all := []topic.Topic{
		{ID: "1", Title: "Old topic"},
		{ID: "2", Title: "Current topic"},
	}
	out := tp.renderRecent(all, cur, true, 30)
	if !strings.Contains(out, "▸") {
		t.Errorf("renderRecent() should contain selection arrow, got: %s", out)
	}
}

func TestTopicPanel_RenderPopupEmpty(t *testing.T) {
	tp := &topicPanel{
		filtered: nil,
		selIdx:   0,
		view:     viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
	}
	out := tp.renderPopup(40, nil)
	if !strings.Contains(out, "No matching topics") {
		t.Errorf("renderPopup() should contain 'No matching topics', got: %s", out)
	}
}

func TestTopicPanel_RenderPopupWithTopics(t *testing.T) {
	tp := &topicPanel{
		filtered: []topic.Topic{
			{ID: "a", Title: "Alpha"},
			{ID: "b", Title: "Beta"},
		},
		selIdx: 0,
		view:   viewport.New(viewport.WithWidth(40), viewport.WithHeight(10)),
	}
	out := tp.renderPopup(40, nil)
	if !strings.Contains(out, "ALL TOPICS") {
		t.Errorf("renderPopup() should contain 'ALL TOPICS', got: %s", out)
	}
	if !strings.Contains(out, "Alpha") {
		t.Errorf("renderPopup() should contain 'Alpha', got: %s", out)
	}
}

func TestTopicPanel_FilterTopics(t *testing.T) {
	tp := &topicPanel{}
	all := []topic.Topic{
		{ID: "1", Title: "Machine Learning"},
		{ID: "2", Title: "Web Development"},
	}
	tp.filter = "machine"
	tp.filterTopics(all)
	if len(tp.filtered) != 1 {
		t.Errorf("expected 1 filtered topic, got %d", len(tp.filtered))
	}
	if tp.filtered[0].ID != "1" {
		t.Errorf("expected topic ID '1', got %q", tp.filtered[0].ID)
	}
}

func TestTopicPanel_FilterTopicsEmpty(t *testing.T) {
	tp := &topicPanel{filter: ""}
	all := []topic.Topic{
		{ID: "1", Title: "A"},
		{ID: "2", Title: "B"},
	}
	tp.filterTopics(all)
	if len(tp.filtered) != 2 {
		t.Errorf("expected 2 topics with empty filter, got %d", len(tp.filtered))
	}
}

func TestTopicPanel_FilterTopicsNoMatch(t *testing.T) {
	tp := &topicPanel{filter: "zzz"}
	all := []topic.Topic{
		{ID: "1", Title: "Hello"},
	}
	tp.filterTopics(all)
	if len(tp.filtered) != 0 {
		t.Errorf("expected 0 matched topics, got %d", len(tp.filtered))
	}
}
