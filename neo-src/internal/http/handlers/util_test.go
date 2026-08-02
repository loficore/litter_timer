package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestHandleFrontendLog_ValidEntry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := strings.NewReader(`{"category":"test","level":"info","message":"test message","runtime":"browser"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/log", body)

	FrontendLog(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}
	// Verify no log text leaked into the response body.
	bodyStr := w.Body.String()
	if strings.Contains(bodyStr, "test message") {
		t.Errorf("response body contains log text, want pure JSON: %s", bodyStr)
	}
}

func TestHandleFrontendLog_EmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/log", strings.NewReader(""))

	FrontendLog(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["success"] != false {
		t.Errorf("success = %v, want false for empty body", got["success"])
	}
	if got["err"] != "empty body" {
		t.Errorf("err = %v, want 'empty body'", got["err"])
	}
}

func TestHandleFrontendLog_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/log", strings.NewReader("not json"))

	FrontendLog(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["success"] != false {
		t.Errorf("success = %v, want false for invalid JSON", got["success"])
	}
	if got["err"] != "invalid json" {
		t.Errorf("err = %v, want 'invalid json'", got["err"])
	}
}

func TestHandleFrontendLog_MissingLevel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := strings.NewReader(`{"category":"test","message":"test message","runtime":"browser"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/log", body)

	FrontendLog(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}
	// Verify no log text leaked into the response body.
	bodyStr := w.Body.String()
	if strings.Contains(bodyStr, "test message") {
		t.Errorf("response body contains log text, want pure JSON: %s", bodyStr)
	}
}

func TestHandleFrontendLog_ErrorLevel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := strings.NewReader(`{"category":"test","level":"error","message":"error message","runtime":"browser"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/log", body)

	FrontendLog(c)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if got["success"] != true {
		t.Errorf("success = %v, want true", got["success"])
	}
	// Verify no log text leaked into the response body.
	bodyStr := w.Body.String()
	if strings.Contains(bodyStr, "error message") {
		t.Errorf("response body contains log text, want pure JSON: %s", bodyStr)
	}
}

func TestJsonUnmarshal_ValidJSON(t *testing.T) {
	var result map[string]any
	err := jsonUnmarshal([]byte(`{"key":"value"}`), &result)
	if err != nil {
		t.Fatalf("jsonUnmarshal failed: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("result = %v, want value", result["key"])
	}
}

func TestJsonUnmarshal_InvalidJSON(t *testing.T) {
	var result map[string]any
	err := jsonUnmarshal([]byte(`not json`), &result)
	if err == nil {
		t.Error("jsonUnmarshal should fail for invalid JSON")
	}
}

func TestJsonUnmarshal_EmptyJSON(t *testing.T) {
	var result map[string]any
	err := jsonUnmarshal([]byte(`{}`), &result)
	if err != nil {
		t.Fatalf("jsonUnmarshal failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestValidBackupName_EmptyString(t *testing.T) {
	if validBackupName("") {
		t.Error("validBackupName should return false for empty string")
	}
}

func TestValidBackupName_AbsolutePath(t *testing.T) {
	if validBackupName("/absolute") {
		t.Error("validBackupName should reject absolute paths starting with /")
	}
	if validBackupName("\\absolute") {
		t.Error("validBackupName should reject absolute paths starting with \\")
	}
}

func TestValidBackupName_ControlCharacters(t *testing.T) {
	if validBackupName("back\x00up.db") {
		t.Error("validBackupName should reject NULL character")
	}
	if validBackupName("back\x1fab") {
		t.Error("validBackupName should reject DEL character")
	}
}

func TestValidBackupName_PathTraversal(t *testing.T) {
	if validBackupName("../etc/passwd") {
		t.Error("validBackupName should reject path traversal")
	}
}

func TestValidBackupName_Slashes(t *testing.T) {
	if validBackupName("backup/name") {
		t.Error("validBackupName should reject forward slashes")
	}
	if validBackupName("backup\\name") {
		t.Error("validBackupName should reject backslashes")
	}
}

func TestValidBackupName_ValidNames(t *testing.T) {
	validNames := []string{
		"backup_2024_01_01.tar.gz",
		"backup-2024.db",
		"test_backup_123.zip",
		"my.backup.file",
	}

	for _, name := range validNames {
		if !validBackupName(name) {
			t.Errorf("validBackupName(%q) should return true", name)
		}
	}
}

func TestPathID_ValidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/habits/123", nil)
	c.Params = gin.Params{{Key: "id", Value: "123"}}

	id, err := pathID(c, "/api/habits/")
	if err != nil {
		t.Fatalf("pathID failed: %v", err)
	}
	if id != 123 {
		t.Errorf("pathID = %d, want 123", id)
	}
}

func TestPathID_EmptyID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/habits/", nil)
	c.Params = gin.Params{{Key: "id", Value: ""}}

	_, err := pathID(c, "/api/habits/")
	if err != errInvalidID {
		t.Errorf("pathID should return errInvalidID, got %v", err)
	}
}

func TestPathID_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/habits/abc", nil)
	c.Params = gin.Params{{Key: "id", Value: "abc"}}

	_, err := pathID(c, "/api/habits/")
	if err == nil {
		t.Error("pathID should fail for non-numeric ID")
	}
}

func TestPathID_NegativeID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/habits/-456", nil)
	c.Params = gin.Params{{Key: "id", Value: "-456"}}

	id, err := pathID(c, "/api/habits/")
	if err != nil {
		t.Fatalf("pathID failed: %v", err)
	}
	if id != -456 {
		t.Errorf("pathID = %d, want -456", id)
	}
}

func TestPathIDWithSuffix_ValidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/habits/123/detail", nil)

	id, err := pathIDWithSuffix(c, "/api/habits/", "/detail")
	if err != nil {
		t.Fatalf("pathIDWithSuffix failed: %v", err)
	}
	if id != 123 {
		t.Errorf("pathIDWithSuffix = %d, want 123", id)
	}
}

func TestPathIDWithSuffix_EmptyTail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/habits//detail", nil)

	_, err := pathIDWithSuffix(c, "/api/habits/", "/detail")
	if err != errInvalidID {
		t.Errorf("pathIDWithSuffix should return errInvalidID, got %v", err)
	}
}

func TestPathIDWithSuffix_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/habits/abc/detail", nil)

	_, err := pathIDWithSuffix(c, "/api/habits/", "/detail")
	if err == nil {
		t.Error("pathIDWithSuffix should fail for non-numeric ID")
	}
}

func TestNowUnix(t *testing.T) {
	expected := time.Now().Unix()
	got := nowUnix()

	diff := got - expected
	if diff < -1 || diff > 1 {
		t.Errorf("nowUnix = %d, want approximately %d", got, expected)
	}
}

func TestNowUnix_Positive(t *testing.T) {
	got := nowUnix()
	if got <= 0 {
		t.Errorf("nowUnix should return positive value, got %d", got)
	}
}

func TestErrInvalidID(t *testing.T) {
	if errInvalidID == nil {
		t.Error("errInvalidID should not be nil")
	}
	if errInvalidID.Error() != "invalid id" {
		t.Errorf("errInvalidID.Error() = %q, want 'invalid id'", errInvalidID.Error())
	}
}

func TestMaskFunction_EmptyString(t *testing.T) {
	if mask("") != "" {
		t.Errorf("mask(\"\") = %q, want \"\"", mask(""))
	}
}

func TestMaskFunction_NonEmptyString(t *testing.T) {
	if mask("secret") != "******" {
		t.Errorf("mask(\"secret\") = %q, want \"******\"", mask("secret"))
	}
}

func TestMaskFunction_LongString(t *testing.T) {
	if mask("very_long_secret_key") != "******" {
		t.Errorf("mask(\"very_long_secret_key\") = %q, want \"******\"", mask("very_long_secret_key"))
	}
}

func TestMasterPasswordErrorFunction(t *testing.T) {
	result := masterPasswordError("test_code", "test message", "setup")

	if result["success"] != false {
		t.Errorf("success = %v, want false", result["success"])
	}
	if result["error"] != "test_code" {
		t.Errorf("error = %v, want test_code", result["error"])
	}
	if result["message"] != "test message" {
		t.Errorf("message = %v, want 'test message'", result["message"])
	}

	action, ok := result["action"].(gin.H)
	if !ok {
		t.Fatal("action should be a gin.H")
	}
	if action["type"] != "show_modal" {
		t.Errorf("action.type = %v, want show_modal", action["type"])
	}
	if action["target"] != "master_password" {
		t.Errorf("action.target = %v, want master_password", action["target"])
	}
	params, _ := action["params"].(gin.H)
	if params["mode"] != "setup" {
		t.Errorf("action.params.mode = %v, want setup", params["mode"])
	}
}

func TestMasterPasswordErrorFunction_EmptyMode(t *testing.T) {
	result := masterPasswordError("test_code", "test message", "")

	action, ok := result["action"].(gin.H)
	if !ok {
		t.Fatal("action should be a gin.H")
	}
	if _, exists := action["params"]; exists {
		t.Error("action.params should not exist when mode is empty")
	}
}
