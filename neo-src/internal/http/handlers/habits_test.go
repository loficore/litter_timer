package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"little-timer/internal/http/app"
)

// =============================================================================
// /api/habit-sets
// =============================================================================

func TestHandleHabitSetCreate(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/habit-sets",
		strings.NewReader(`{"name":"Work","description":"work habits","color":"#ff0000"}`))
	c.Set("app", a)

	HabitSetCreate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["name"] != "Work" {
		t.Errorf("name = %v, want Work", got["name"])
	}
	if got["color"] != "#ff0000" {
		t.Errorf("color = %v, want #ff0000", got["color"])
	}
	if got["id"] == nil {
		t.Error("missing id in response")
	}
}

func TestHandleHabitSetCreate_MissingName(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/habit-sets",
		strings.NewReader(`{"color":"#ff0000"}`))
	c.Set("app", a)

	HabitSetCreate(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestHandleHabitSetCreate_DefaultColor(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/habit-sets",
		strings.NewReader(`{"name":"NoColor"}`))
	c.Set("app", a)

	HabitSetCreate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["color"] != "#6366f1" {
		t.Errorf("default color = %v, want #6366f1", got["color"])
	}
}

func TestHandleHabitSetList(t *testing.T) {
	a := newTestApp(t)

	// Seed two habit sets.
	for _, name := range []string{"A", "B"} {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/habit-sets",
			strings.NewReader(`{"name":"`+name+`"}`))
		c.Set("app", a)
		HabitSetCreate(c)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/habit-sets", nil)
	c.Set("app", a)

	HabitSetList(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
	var got []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d habit sets, want 2", len(got))
	}
	for i, row := range got {
		for _, field := range []string{"id", "name", "description", "color"} {
			if _, ok := row[field]; !ok {
				t.Errorf("row[%d]: missing field %q", i, field)
			}
		}
	}
}

func TestHandleHabitSetUpdate(t *testing.T) {
	a := newTestApp(t)

	// Create the set first.
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/habit-sets",
		strings.NewReader(`{"name":"Old","color":"#000000"}`))
	c.Set("app", a)
	HabitSetCreate(c)
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	id := int64(created["id"].(float64))

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodPut, "/api/habit-sets/"+itoa(id),
		strings.NewReader(`{"name":"New","description":"updated","color":"#ffffff","wallpaper":"foo.png"}`))
	c2.Params = gin.Params{{Key: "id", Value: itoa(id)}}
	c2.Set("app", a)

	HabitSetUpdate(c2)

	if w2.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w2.Code, w2.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w2.Body.Bytes(), &got)
	if got["name"] != "New" {
		t.Errorf("name = %v, want New", got["name"])
	}
	if got["wallpaper"] != "foo.png" {
		t.Errorf("wallpaper = %v, want foo.png", got["wallpaper"])
	}
}

func TestHandleHabitSetUpdate_InvalidID(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/habit-sets/abc",
		strings.NewReader(`{"name":"X"}`))
	c.Params = gin.Params{{Key: "id", Value: "abc"}}
	c.Set("app", a)

	HabitSetUpdate(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestHandleHabitSetUpdate_MissingName(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/habit-sets/1",
		strings.NewReader(`{"color":"#fff"}`))
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Set("app", a)

	HabitSetUpdate(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestHandleHabitSetDelete(t *testing.T) {
	a := newTestApp(t)

	// Create the set first.
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/habit-sets",
		strings.NewReader(`{"name":"ToDelete"}`))
	c.Set("app", a)
	HabitSetCreate(c)
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	id := int64(created["id"].(float64))

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodDelete, "/api/habit-sets/"+itoa(id), nil)
	c2.Params = gin.Params{{Key: "id", Value: itoa(id)}}
	c2.Set("app", a)

	HabitSetDelete(c2)

	if w2.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w2.Code, w2.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w2.Body.Bytes(), &got)
	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}
}

func TestHandleHabitSetDelete_InvalidID(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/habit-sets/abc", nil)
	c.Params = gin.Params{{Key: "id", Value: "abc"}}
	c.Set("app", a)

	HabitSetDelete(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

// =============================================================================
// /api/habits
// =============================================================================

func newHabitSetID(t *testing.T, a *app.App, name string) int64 {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/habit-sets",
		strings.NewReader(`{"name":"`+name+`"}`))
	c.Set("app", a)
	HabitSetCreate(c)
	if w.Code != http.StatusOK {
		t.Fatalf("seed habit-set %q: code = %d, body = %s", name, w.Code, w.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	return int64(created["id"].(float64))
}

func TestHandleHabitCreate(t *testing.T) {
	a := newTestApp(t)
	setID := newHabitSetID(t, a, "Work")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/habits",
		strings.NewReader(`{"set_id":`+itoa(setID)+`,"name":"Reading","goal_seconds":600,"color":"#00ff00"}`))
	c.Set("app", a)

	HabitCreate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["name"] != "Reading" {
		t.Errorf("name = %v, want Reading", got["name"])
	}
	if got["goal_seconds"] != float64(600) {
		t.Errorf("goal_seconds = %v, want 600", got["goal_seconds"])
	}
}

func TestHandleHabitCreate_MissingName(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/habits",
		strings.NewReader(`{"set_id":1}`))
	c.Set("app", a)

	HabitCreate(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestHandleHabitCreate_DefaultGoal(t *testing.T) {
	a := newTestApp(t)
	setID := newHabitSetID(t, a, "Defaults")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/habits",
		strings.NewReader(`{"set_id":`+itoa(setID)+`,"name":"DefaultGoal"}`))
	c.Set("app", a)

	HabitCreate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["goal_seconds"] != float64(1500) {
		t.Errorf("default goal_seconds = %v, want 1500", got["goal_seconds"])
	}
}

func TestHandleHabitList(t *testing.T) {
	a := newTestApp(t)
	setID := newHabitSetID(t, a, "List")
	for _, name := range []string{"H1", "H2"} {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/habits",
			strings.NewReader(`{"set_id":`+itoa(setID)+`,"name":"`+name+`"}`))
		c.Set("app", a)
		HabitCreate(c)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/habits", nil)
	c.Set("app", a)

	HabitList(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
	var got []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d habits, want 2", len(got))
	}
	for i, row := range got {
		for _, field := range []string{"id", "set_id", "name", "goal_seconds", "color"} {
			if _, ok := row[field]; !ok {
				t.Errorf("row[%d]: missing field %q", i, field)
			}
		}
	}
}

func TestHandleHabitList_FilterBySet(t *testing.T) {
	a := newTestApp(t)
	setA := newHabitSetID(t, a, "A")
	setB := newHabitSetID(t, a, "B")

	for _, p := range []struct {
		setID int64
		name  string
	}{
		{setA, "hA1"}, {setA, "hA2"}, {setB, "hB1"},
	} {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/habits",
			strings.NewReader(`{"set_id":`+itoa(p.setID)+`,"name":"`+p.name+`"}`))
		c.Set("app", a)
		HabitCreate(c)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/habits?set_id="+itoa(setA), nil)
	c.Set("app", a)

	HabitList(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
	var got []map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if len(got) != 2 {
		t.Errorf("got %d habits in set A, want 2", len(got))
	}
}

func TestHandleHabitList_InvalidSetID(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/habits?set_id=notanumber", nil)
	c.Set("app", a)

	HabitList(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func newHabitID(t *testing.T, a *app.App, setID int64, name string) int64 {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/habits",
		strings.NewReader(`{"set_id":`+itoa(setID)+`,"name":"`+name+`"}`))
	c.Set("app", a)
	HabitCreate(c)
	if w.Code != http.StatusOK {
		t.Fatalf("seed habit %q: code = %d, body = %s", name, w.Code, w.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	return int64(created["id"].(float64))
}

func TestHandleHabitUpdate(t *testing.T) {
	a := newTestApp(t)
	setID := newHabitSetID(t, a, "Update")
	habitID := newHabitID(t, a, setID, "Before")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/habits/"+itoa(habitID),
		strings.NewReader(`{"name":"After","goal_seconds":3000,"color":"#abcdef","wallpaper":"bar.jpg"}`))
	c.Params = gin.Params{{Key: "id", Value: itoa(habitID)}}
	c.Set("app", a)

	HabitUpdate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["name"] != "After" {
		t.Errorf("name = %v, want After", got["name"])
	}
	if got["goal_seconds"] != float64(3000) {
		t.Errorf("goal_seconds = %v, want 3000", got["goal_seconds"])
	}
}

func TestHandleHabitUpdate_InvalidID(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/habits/abc",
		strings.NewReader(`{"name":"X"}`))
	c.Params = gin.Params{{Key: "id", Value: "abc"}}
	c.Set("app", a)

	HabitUpdate(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestHandleHabitUpdate_MissingName(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/habits/1",
		strings.NewReader(`{"goal_seconds":100}`))
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Set("app", a)

	HabitUpdate(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

// =============================================================================
// Dedup Tests
// =============================================================================

func TestHandleHabitCreate_DuplicateName(t *testing.T) {
	a := newTestApp(t)
	setID := newHabitSetID(t, a, "DedupSet")
	// Create first habit
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/habits",
		strings.NewReader(`{"set_id":`+itoa(setID)+`,"name":"Read","goal_seconds":600}`))
	c.Set("app", a)
	HabitCreate(c)
	if w.Code != http.StatusOK {
		t.Fatalf("first create: %d", w.Code)
	}
	// Try duplicate name in same set
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodPost, "/api/habits",
		strings.NewReader(`{"set_id":`+itoa(setID)+`,"name":"Read","goal_seconds":600}`))
	c2.Set("app", a)
	HabitCreate(c2)
	if w2.Code != http.StatusBadRequest {
		t.Errorf("duplicate: got %d, want 400", w2.Code)
	}
}

func TestHandleHabitCreate_DuplicateNameCrossSet(t *testing.T) {
	a := newTestApp(t)
	setID1 := newHabitSetID(t, a, "DedupSet1")
	setID2 := newHabitSetID(t, a, "DedupSet2")
	// Create habit in set1
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/habits",
		strings.NewReader(`{"set_id":`+itoa(setID1)+`,"name":"Read","goal_seconds":600}`))
	c.Set("app", a)
	HabitCreate(c)
	if w.Code != http.StatusOK {
		t.Fatalf("first create: %d", w.Code)
	}
	// Same name in set2 → should succeed
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodPost, "/api/habits",
		strings.NewReader(`{"set_id":`+itoa(setID2)+`,"name":"Read","goal_seconds":600}`))
	c2.Set("app", a)
	HabitCreate(c2)
	if w2.Code != http.StatusOK {
		t.Errorf("cross-set duplicate: got %d, want 200", w2.Code)
	}
}

func TestHandleHabitUpdate_DuplicateName(t *testing.T) {
	a := newTestApp(t)
	setID := newHabitSetID(t, a, "UpdDedupSet")
	// Create two habits
	habit1ID := newHabitID(t, a, setID, "Book")
	_ = newHabitID(t, a, setID, "Movie")

	// Update habit1's name to habit2's name
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/habits/"+itoa(habit1ID),
		strings.NewReader(`{"name":"Movie","goal_seconds":600}`))
	c.Set("app", a)
	HabitUpdate(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("update to duplicate: got %d, want 400", w.Code)
	}
}

func TestHandleHabitDelete(t *testing.T) {
	a := newTestApp(t)
	setID := newHabitSetID(t, a, "Del")
	habitID := newHabitID(t, a, setID, "DeleteMe")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/habits/"+itoa(habitID), nil)
	c.Params = gin.Params{{Key: "id", Value: itoa(habitID)}}
	c.Set("app", a)

	HabitDelete(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}
}

func TestHandleHabitDelete_InvalidID(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/habits/abc", nil)
	c.Params = gin.Params{{Key: "id", Value: "abc"}}
	c.Set("app", a)

	HabitDelete(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestHandleHabitDetail(t *testing.T) {
	a := newTestApp(t)
	setID := newHabitSetID(t, a, "Detail")
	habitID := newHabitID(t, a, setID, "DetailMe")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/habits/"+itoa(habitID)+"/detail", nil)
	c.Set("app", a)

	HabitDetail(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["name"] != "DetailMe" {
		t.Errorf("name = %v, want DetailMe", got["name"])
	}
	if _, ok := got["today_seconds"]; !ok {
		t.Error("missing today_seconds in detail response")
	}
	if _, ok := got["progress_percent"]; !ok {
		t.Error("missing progress_percent in detail response")
	}
}

func TestHandleHabitDetail_InvalidID(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/habits/abc/detail", nil)
	c.Set("app", a)

	HabitDetail(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestHandleHabitDetail_NotFound(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/habits/999999/detail", nil)
	c.Set("app", a)

	HabitDetail(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", w.Code)
	}
}

// =============================================================================
// /api/sessions
// =============================================================================

func TestHandleSessionCreate(t *testing.T) {
	a := newTestApp(t)
	setID := newHabitSetID(t, a, "SessCreate")
	habitID := newHabitID(t, a, setID, "SessHabit")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/sessions",
		strings.NewReader(`{"habit_id":`+itoa(habitID)+`,"duration_seconds":1200,"count":2}`))
	c.Set("app", a)

	SessionCreate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["duration_seconds"] != float64(1200) {
		t.Errorf("duration_seconds = %v, want 1200", got["duration_seconds"])
	}
	if _, ok := got["date"]; !ok {
		t.Error("missing date in response")
	}
}

func TestHandleSessionCreate_DefaultCount(t *testing.T) {
	a := newTestApp(t)
	setID := newHabitSetID(t, a, "SessDefault")
	habitID := newHabitID(t, a, setID, "SessDefaultHabit")

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/sessions",
		strings.NewReader(`{"habit_id":`+itoa(habitID)+`,"duration_seconds":600}`))
	c.Set("app", a)

	SessionCreate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
	// count default is applied server-side; success is enough here.
}

func TestHandleSessionList_Today(t *testing.T) {
	a := newTestApp(t)
	setID := newHabitSetID(t, a, "SessList")
	habitID := newHabitID(t, a, setID, "SessListHabit")

	// Seed a session.
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/sessions",
		strings.NewReader(`{"habit_id":`+itoa(habitID)+`,"duration_seconds":300,"count":1}`))
	c.Set("app", a)
	SessionCreate(c)

	// List with no filter → today.
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	c2.Set("app", a)
	SessionList(c2)

	if w2.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w2.Code, w2.Body.String())
	}
	var got []map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if len(got) == 0 {
		t.Error("expected at least one session today, got 0")
	}
	for i, row := range got {
		for _, field := range []string{"id", "habit_id", "duration_seconds", "date"} {
			if _, ok := row[field]; !ok {
				t.Errorf("row[%d]: missing field %q", i, field)
			}
		}
	}
}

func TestHandleSessionList_ByDate(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/sessions?date=2024-01-01", nil)
	c.Set("app", a)
	SessionList(c)

	if w.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", w.Code)
	}
}

func TestHandleSessionList_ByRange(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/sessions?start_date=2024-01-01&end_date=2024-12-31", nil)
	c.Set("app", a)
	SessionList(c)

	if w.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", w.Code)
	}
}

// =============================================================================
// /api/timer-sessions
// =============================================================================

func TestHandleTimerSessionCreate(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/timer-sessions",
		strings.NewReader(`{"mode":"countdown","work_duration":1800,"rest_duration":300,"loop_count":4}`))
	c.Set("app", a)

	TimerSessionCreate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["id"] == nil {
		t.Error("missing id in response")
	}
}

func TestHandleTimerSessionCreate_Defaults(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/timer-sessions",
		strings.NewReader(`{}`))
	c.Set("app", a)

	TimerSessionCreate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got["id"] == nil {
		t.Error("missing id in response")
	}
}

func TestHandleTimerSessionList_NoActive(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/timer-sessions", nil)
	c.Set("app", a)

	TimerSessionList(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", w.Code)
	}
}

func TestHandleTimerSessionList_ByID(t *testing.T) {
	a := newTestApp(t)

	// Create a session.
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/timer-sessions",
		strings.NewReader(`{"mode":"stopwatch","work_duration":600}`))
	c.Set("app", a)
	TimerSessionCreate(c)
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	id := int64(created["id"].(float64))

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/timer-sessions?id="+itoa(id), nil)
	c2.Set("app", a)
	TimerSessionList(c2)

	if w2.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w2.Code, w2.Body.String())
	}
	// The response uses uppercase field names ("ID" not "id").
	var got map[string]any
	_ = json.Unmarshal(w2.Body.Bytes(), &got)
	if int64(got["id"].(float64)) != id {
		t.Errorf("id = %v, want %d", got["id"], id)
	}
}

func TestHandleTimerSessionList_InvalidID(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/timer-sessions?id=notanumber", nil)
	c.Set("app", a)

	TimerSessionList(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestHandleTimerSessionList_NotFound(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/timer-sessions?id=999999", nil)
	c.Set("app", a)

	TimerSessionList(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", w.Code)
	}
}

func TestHandleTimerSessionUpdate(t *testing.T) {
	a := newTestApp(t)

	// Create a session to update.
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/timer-sessions",
		strings.NewReader(`{"mode":"stopwatch","work_duration":600}`))
	c.Set("app", a)
	TimerSessionCreate(c)
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	id := int64(created["id"].(float64))

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodPut, "/api/timer-sessions/"+itoa(id),
		strings.NewReader(`{"elapsed_seconds":42,"paused_total_seconds":0,"is_running":true,"is_paused":false,"is_finished":false,"current_round":1,"in_rest":false}`))
	c2.Params = gin.Params{{Key: "id", Value: itoa(id)}}
	c2.Set("app", a)

	TimerSessionUpdate(c2)

	if w2.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w2.Code, w2.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w2.Body.Bytes(), &got)
	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}
}

func TestHandleTimerSessionUpdate_InvalidID(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/timer-sessions/abc",
		strings.NewReader(`{"elapsed_seconds":1,"is_running":true,"is_paused":false,"is_finished":false}`))
	c.Params = gin.Params{{Key: "id", Value: "abc"}}
	c.Set("app", a)

	TimerSessionUpdate(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestHandleTimerSessionDelete(t *testing.T) {
	a := newTestApp(t)

	// Create a session to delete.
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/timer-sessions",
		strings.NewReader(`{"mode":"stopwatch","work_duration":600}`))
	c.Set("app", a)
	TimerSessionCreate(c)
	var created map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	id := int64(created["id"].(float64))

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodDelete, "/api/timer-sessions/"+itoa(id), nil)
	c2.Params = gin.Params{{Key: "id", Value: itoa(id)}}
	c2.Set("app", a)

	TimerSessionDelete(c2)

	if w2.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w2.Code, w2.Body.String())
	}
	var got map[string]any
	_ = json.Unmarshal(w2.Body.Bytes(), &got)
	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}
}

func TestHandleTimerSessionDelete_InvalidID(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/timer-sessions/abc", nil)
	c.Params = gin.Params{{Key: "id", Value: "abc"}}
	c.Set("app", a)

	TimerSessionDelete(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

// =============================================================================
// Helpers
// =============================================================================

func itoa(i int64) string {
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = digits[i%10]
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

// =============================================================================
// /api/sessions DELETE & pagination
// =============================================================================

func TestHandleSessionDelete(t *testing.T) {
	a := newTestApp(t)
	setID := newHabitSetID(t, a, "DelSession")
	habitID := newHabitID(t, a, setID, "DelHabit")

	// Create session via handler
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/sessions",
		strings.NewReader(`{"habit_id":`+itoa(habitID)+`,"duration_seconds":300,"count":1}`))
	c.Set("app", a)
	SessionCreate(c)
	if w.Code != http.StatusOK {
		t.Fatalf("create failed: code=%d body=%s", w.Code, w.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	sessionID := int64(created["id"].(float64))

	// Delete it
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodDelete, "/api/sessions/"+itoa(sessionID), nil)
	c2.Params = gin.Params{{Key: "id", Value: itoa(sessionID)}}
	c2.Set("app", a)
	SessionDelete(c2)

	if w2.Code != http.StatusNoContent {
		t.Errorf("code = %d, want 204", w2.Code)
	}
}

func TestHandleSessionDelete_NotFound(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/sessions/99999", nil)
	c.Params = gin.Params{{Key: "id", Value: "99999"}}
	c.Set("app", a)
	SessionDelete(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", w.Code)
	}
}

func TestHandleSessionList_Pagination(t *testing.T) {
	a := newTestApp(t)
	setID := newHabitSetID(t, a, "SessPag")
	habitID := newHabitID(t, a, setID, "SessPagHabit")

	// Create 3 sessions
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/sessions",
			strings.NewReader(`{"habit_id":`+itoa(habitID)+`,"duration_seconds":100,"count":1}`))
		c.Set("app", a)
		SessionCreate(c)
		if w.Code != http.StatusOK {
			t.Fatalf("create failed: code=%d body=%s", w.Code, w.Body.String())
		}
	}

	// List with limit=2
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/sessions?limit=2", nil)
	c2.Set("app", a)
	SessionList(c2)

	if w2.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w2.Code, w2.Body.String())
	}
	var rows []map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Errorf("limit=2 should return exactly 2 sessions, got %d", len(rows))
	}
}

// =============================================================================
// Pagination Tests
// =============================================================================

func TestHandleHabitList_Pagination(t *testing.T) {
	a := newTestApp(t)
	setID := newHabitSetID(t, a, "HabPagSet")
	// Create 3 habits
	for _, name := range []string{"HabPag1", "HabPag2", "HabPag3"} {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/habits",
			strings.NewReader(`{"set_id":`+itoa(setID)+`,"name":"`+name+`","goal_seconds":600}`))
		c.Set("app", a)
		HabitCreate(c)
		if w.Code != http.StatusOK {
			t.Fatalf("create habit: %d", w.Code)
		}
	}
	// List with limit=2
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/habits?limit=2", nil)
	c2.Set("app", a)
	HabitList(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("list: %d", w2.Code)
	}
	var rows []map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &rows); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("limit=2: got %d rows, want 2", len(rows))
	}
}

func TestHandleHabitSetList_Pagination(t *testing.T) {
	a := newTestApp(t)
	for _, name := range []string{"PagSet1", "PagSet2"} {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/habit-sets",
			strings.NewReader(`{"name":"`+name+`"}`))
		c.Set("app", a)
		HabitSetCreate(c)
		if w.Code != http.StatusOK {
			t.Fatalf("create set: %d", w.Code)
		}
	}
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/habit-sets?limit=1", nil)
	c2.Set("app", a)
	HabitSetList(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("list: %d", w2.Code)
	}
	var rows []map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &rows); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("limit=1: got %d rows, want 1", len(rows))
	}
}

func TestHandleSessionList_PaginationInvalid(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/sessions?limit=invalid", nil)
	c.Set("app", a)
	SessionList(c)
	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", w.Code)
	}
}

func TestHandleHabitStats(t *testing.T) {
	a := newTestApp(t)
	setID := newHabitSetID(t, a, "StatsSet")
	habitID := newHabitID(t, a, setID, "StatsHabit")

	// Create a session
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/sessions",
		strings.NewReader(`{"habit_id":`+itoa(habitID)+`,"duration_seconds":3600,"count":1}`))
	c.Set("app", a)
	SessionCreate(c)
	if w.Code != http.StatusOK {
		t.Fatalf("create session: %d", w.Code)
	}

	// Get stats
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/habits/"+itoa(habitID)+"/stats", nil)
	c2.Set("app", a)
	HabitStats(c2)

	if w2.Code != http.StatusOK {
		t.Fatalf("stats: got %d, want 200: %s", w2.Code, w2.Body.String())
	}
	var stats map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &stats); err != nil {
		t.Fatalf("not JSON: %v", err)
	}

	requiredFields := []string{"total_seconds_week", "total_sessions_week", "weekly_breakdown"}
	for _, field := range requiredFields {
		val, ok := stats[field]
		if !ok {
			t.Errorf("missing field %q in stats response", field)
		} else if val == nil {
			t.Errorf("field %q is null, want non-nil value", field)
		}
	}

	if wb, ok := stats["weekly_breakdown"].(map[string]interface{}); ok {
		_ = wb
	} else if stats["weekly_breakdown"] != nil {
		t.Errorf("weekly_breakdown = %T, want map", stats["weekly_breakdown"])
	}
}

func TestHandleHabitStats_NotFound(t *testing.T) {
	a := newTestApp(t)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/habits/99999/stats", nil)
	c.Set("app", a)
	HabitStats(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", w.Code)
	}
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["success"] != false {
		t.Errorf("success = %v, want false", got["success"])
	}
}
