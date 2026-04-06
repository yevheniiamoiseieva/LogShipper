package tui

import (
	"testing"
	"time"

	"collector/internal/event"
	"collector/internal/graph"
)

func TestRiskScore_Zero(t *testing.T) {
	if got := riskScore(0, 0, 0); got != 0 {
		t.Errorf("riskScore(0,0,0) = %v, want 0", got)
	}
}

func TestRiskScore_Anomalies(t *testing.T) {
	if got := riskScore(3, 0, 0); got != 6.0 {
		t.Errorf("got %v, want 6.0", got)
	}
}

func TestRiskScore_ErrorRate(t *testing.T) {
	if got := riskScore(0, 0.5, 0); got != 5.0 {
		t.Errorf("got %v, want 5.0", got)
	}
}

func TestRiskScore_Latency(t *testing.T) {
	if got := riskScore(0, 0, 200*time.Millisecond); got != 2.0 {
		t.Errorf("got %v, want 2.0", got)
	}
}

func TestRiskScore_Combined(t *testing.T) {
	if got := riskScore(2, 0.1, 300*time.Millisecond); got != 8.0 {
		t.Errorf("got %v, want 8.0", got)
	}
}

func TestRiskStyle_Low(t *testing.T) {
	if RiskStyle(1.0).GetForeground() != colorGreen {
		t.Error("risk < 3 should be green")
	}
}

func TestRiskStyle_Mid(t *testing.T) {
	if RiskStyle(5.0).GetForeground() != colorYellow {
		t.Error("risk 3-7 should be yellow")
	}
}

func TestRiskStyle_High(t *testing.T) {
	if RiskStyle(8.0).GetForeground() != colorRed {
		t.Error("risk > 7 should be red")
	}
}

func TestRiskStyle_Boundary(t *testing.T) {
	if RiskStyle(3.0).GetForeground() != colorYellow {
		t.Error("risk == 3 should be yellow")
	}
	if RiskStyle(7.0).GetForeground() != colorYellow {
		t.Error("risk == 7 should be yellow")
	}
}

func TestStatusStyle_2xx(t *testing.T) {
	if StatusStyle(200).GetForeground() != colorGreen {
		t.Error("2xx should be green")
	}
}

func TestStatusStyle_4xx(t *testing.T) {
	if StatusStyle(404).GetForeground() != colorYellow {
		t.Error("4xx should be yellow")
	}
}

func TestStatusStyle_5xx(t *testing.T) {
	if StatusStyle(500).GetForeground() != colorRed {
		t.Error("5xx should be red")
	}
}

func TestScreen1_Update_Empty(t *testing.T) {
	s := NewScreen1()
	s.Update(graph.CallGraphSnapshot{}, nil, nil)
	if len(s.rows) != 0 {
		t.Errorf("empty snapshot → 0 rows, got %d", len(s.rows))
	}
}

func TestScreen1_SortByRisk(t *testing.T) {
	s := NewScreen1()
	s.rows = []ServiceRow{
		{Name: "a", RiskScore: 2.0},
		{Name: "b", RiskScore: 9.0},
		{Name: "c", RiskScore: 5.0},
	}
	s.sortCol = SortByRisk
	s.applyFilterAndSort()
	if s.filtered[0].Name != "b" {
		t.Errorf("first row should be 'b', got %s", s.filtered[0].Name)
	}
}

func TestScreen1_Filter(t *testing.T) {
	s := NewScreen1()
	s.rows = []ServiceRow{
		{Name: "auth-service"},
		{Name: "payment-api"},
		{Name: "auth-proxy"},
	}
	s.filterInput.SetValue("auth")
	s.applyFilterAndSort()
	if len(s.filtered) != 2 {
		t.Errorf("filter 'auth' → 2 rows, got %d", len(s.filtered))
	}
}

func TestScreen1_CursorClamp(t *testing.T) {
	s := NewScreen1()
	s.cursor = 10
	s.rows = []ServiceRow{{Name: "only"}}
	s.applyFilterAndSort()
	if s.cursor != 0 {
		t.Errorf("cursor should clamp to 0, got %d", s.cursor)
	}
}

func TestScreen1_SelectedService_Empty(t *testing.T) {
	s := NewScreen1()
	if got := s.SelectedService(); got != "" {
		t.Errorf("got %q, want ''", got)
	}
}

func TestScreen1_SortCycle(t *testing.T) {
	s := NewScreen1()
	for i := 0; i < 5; i++ {
		s.sortCol = (s.sortCol + 1) % 5
	}
	if s.sortCol != SortByRisk {
		t.Errorf("after 5 cycles should be SortByRisk, got %d", s.sortCol)
	}
}

func TestScreen2_SetService(t *testing.T) {
	s := NewScreen2()
	s.SetService("auth")
	if s.service != "auth" {
		t.Errorf("service = %q, want auth", s.service)
	}
	if s.cursor != 0 {
		t.Error("cursor should reset to 0")
	}
}

func TestScreen2_SelectedEdge_Empty(t *testing.T) {
	s := NewScreen2()
	if e := s.SelectedEdge(); e != nil {
		t.Error("empty SelectedEdge() should be nil")
	}
}

func TestScreen2_Update_FiltersCorrectly(t *testing.T) {
	s := NewScreen2()
	s.SetService("auth")
	snap := graph.CallGraphSnapshot{
		Nodes: []string{"auth", "db", "cache"},
		Edges: []graph.Edge{
			{Src: "api", Dst: "auth", Operation: "POST /login"},
			{Src: "auth", Dst: "db", Operation: "SELECT"},
			{Src: "cache", Dst: "unrelated", Operation: "GET"},
		},
		At: time.Now(),
	}
	s.Update(snap)
	if len(s.upstream) != 1 {
		t.Errorf("upstream = %d, want 1", len(s.upstream))
	}
	if len(s.downstream) != 1 {
		t.Errorf("downstream = %d, want 1", len(s.downstream))
	}
}

func TestScreen3_AddEvent_WrongEdge(t *testing.T) {
	s := NewScreen3()
	s.SetEdge("a", "b", "op", "a|b|op")
	s.AddEvent(makeNormEvent("x", "y", "other"))
	if len(s.events) != 0 {
		t.Error("wrong edge event should not be added")
	}
}

func TestScreen3_AddEvent_Matching(t *testing.T) {
	s := NewScreen3()
	s.SetEdge("a", "b", "op", "a|b|op")
	s.AddEvent(makeNormEvent("a", "b", "op"))
	if len(s.events) != 1 {
		t.Errorf("matching event should be added, got %d", len(s.events))
	}
}

func TestScreen3_MaxEvents(t *testing.T) {
	s := NewScreen3()
	s.SetEdge("a", "b", "op", "a|b|op")
	for i := 0; i < 110; i++ {
		s.AddEvent(makeNormEvent("a", "b", "op"))
	}
	if len(s.events) != maxEvents {
		t.Errorf("capped at %d, got %d", maxEvents, len(s.events))
	}
}

func TestScreen3_AutoScroll_MovesToEnd(t *testing.T) {
	s := NewScreen3()
	s.SetEdge("a", "b", "op", "a|b|op")
	s.autoScroll = true
	for i := 0; i < 5; i++ {
		s.AddEvent(makeNormEvent("a", "b", "op"))
	}
	if s.cursor != 4 {
		t.Errorf("autoscroll cursor = %d, want 4", s.cursor)
	}
}

func TestScreen3_AutoScroll_Off(t *testing.T) {
	s := NewScreen3()
	s.SetEdge("a", "b", "op", "a|b|op")
	s.autoScroll = false
	for i := 0; i < 5; i++ {
		s.AddEvent(makeNormEvent("a", "b", "op"))
	}
	if s.cursor != 0 {
		t.Errorf("cursor should stay 0 when autoscroll off, got %d", s.cursor)
	}
}

func TestScreen3_Nil_Event(t *testing.T) {
	s := NewScreen3()
	s.SetEdge("a", "b", "op", "a|b|op")
	s.AddEvent(nil)
	if len(s.events) != 0 {
		t.Error("nil event should not be added")
	}
}

func makeNormEvent(src, dst, op string) *event.NormalizedEvent {
	return &event.NormalizedEvent{
		SrcService: src,
		DstService: dst,
		Operation:  op,
		Timestamp:  time.Now(),
	}
}
