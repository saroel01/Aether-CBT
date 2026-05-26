package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type LoadClient struct {
	BaseURL    string
	TenantID   int
	HTTPClient *http.Client
}

func NewLoadClient(baseURL string, tenantID int) *LoadClient {
	return &LoadClient{
		BaseURL:  strings.TrimRight(baseURL, "/"),
		TenantID: tenantID,
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        500,
				MaxIdleConnsPerHost: 500,
				MaxConnsPerHost:     600,
				IdleConnTimeout:     90 * time.Second,
				DisableKeepAlives:   false,
			},
		},
	}
}

func (c *LoadClient) tenantHeader() string {
	return fmt.Sprintf("%d", c.TenantID)
}

type APIResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
}

type StudentLoginResponse struct {
	PesertaID int    `json:"peserta_id"`
	Token     string `json:"token"`
}

type StartExamResponse struct {
	AttemptToken string `json:"attempt_token"`
}

func (c *LoadClient) StudentLogin(noID, password, token string) (StudentLoginResponse, int, error) {
	body, _ := json.Marshal(map[string]string{
		"no_id":    noID,
		"password": password,
		"token":    token,
	})

	req, err := http.NewRequest("POST", c.BaseURL+"/api/auth/student-login", bytes.NewReader(body))
	if err != nil {
		return StudentLoginResponse{}, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", c.tenantHeader())

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return StudentLoginResponse{}, 0, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return StudentLoginResponse{}, resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncStr(string(raw), 200))
	}

	var apiResp APIResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		return StudentLoginResponse{}, resp.StatusCode, fmt.Errorf("json parse: %v", err)
	}
	if !apiResp.Success {
		return StudentLoginResponse{}, resp.StatusCode, fmt.Errorf("api error: %s", apiResp.Error)
	}

	var loginResp StudentLoginResponse
	if err := json.Unmarshal(apiResp.Data, &loginResp); err != nil {
		var wrapper struct {
			PesertaID int    `json:"peserta_id"`
			Token     string `json:"token"`
		}
		if err2 := json.Unmarshal(apiResp.Data, &wrapper); err2 != nil {
			return StudentLoginResponse{}, resp.StatusCode, fmt.Errorf("parse login data: %v", err)
		}
		loginResp = StudentLoginResponse(wrapper)
	}

	return loginResp, resp.StatusCode, nil
}

func (c *LoadClient) StartExam(jwtToken string, pesertaID, mapelID int) (StartExamResponse, int, error) {
	body, _ := json.Marshal(map[string]int{
		"peserta_id": pesertaID,
		"mapel_id":   mapelID,
	})

	req, err := http.NewRequest("POST", c.BaseURL+"/api/student/start", bytes.NewReader(body))
	if err != nil {
		return StartExamResponse{}, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("X-Tenant-ID", c.tenantHeader())

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return StartExamResponse{}, 0, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return StartExamResponse{}, resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncStr(string(raw), 200))
	}

	var apiResp APIResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		return StartExamResponse{}, resp.StatusCode, err
	}

	var startResp StartExamResponse
	if err := json.Unmarshal(apiResp.Data, &startResp); err != nil {
		return StartExamResponse{}, resp.StatusCode, err
	}

	return startResp, resp.StatusCode, nil
}

func (c *LoadClient) GetRemainingTime(jwtToken string, pesertaID, mapelID int) (int, error) {
	req, err := http.NewRequest("GET",
		fmt.Sprintf("%s/api/student/remaining-time?peserta_id=%d&mapel_id=%d", c.BaseURL, pesertaID, mapelID), nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("X-Tenant-ID", c.tenantHeader())

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncStr(string(raw), 100))
	}

	return resp.StatusCode, nil
}

func (c *LoadClient) UpdateProgress(jwtToken string, pesertaID, mapelID, answeredCount, totalQ int) (int, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"peserta_id":      pesertaID,
		"mapel_id":        mapelID,
		"answered_count":  answeredCount,
		"total_questions": totalQ,
	})

	req, err := http.NewRequest("POST", c.BaseURL+"/api/student/progress", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("X-Tenant-ID", c.tenantHeader())

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	return resp.StatusCode, nil
}

func (c *LoadClient) RecordInfraction(jwtToken string, pesertaID, mapelID int) (int, error) {
	body, _ := json.Marshal(map[string]int{
		"peserta_id": pesertaID,
		"mapel_id":   mapelID,
	})

	req, err := http.NewRequest("POST", c.BaseURL+"/api/student/infraction", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("X-Tenant-ID", c.tenantHeader())

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	return resp.StatusCode, nil
}

func (c *LoadClient) SubmitWebhook(noID, score, totalPoints string, xmlData []byte, attemptToken string) (int, error) {
	form := url.Values{}
	form.Set("sid", noID)
	form.Set("sp", score)
	form.Set("tp", totalPoints)
	form.Set("dr", string(xmlData))
	form.Set("attempt_token", attemptToken)

	var resp *http.Response
	var err error
	body := form.Encode()
	for attempt := 0; attempt < 4; attempt++ {
		req, reqErr := http.NewRequest("POST", c.BaseURL+"/api/ispring/webhook", strings.NewReader(body))
		if reqErr != nil {
			return 0, reqErr
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-Tenant-ID", c.tenantHeader())

		resp, err = c.HTTPClient.Do(req)
		if err == nil {
			break
		}
		time.Sleep(time.Duration(25*(attempt+1)) * time.Millisecond)
	}
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncStr(string(raw), 200))
	}

	return resp.StatusCode, nil
}

func (c *LoadClient) HealthCheck() error {
	resp, err := c.HTTPClient.Get(c.BaseURL + "/api/health")
	if err != nil {
		return fmt.Errorf("server unreachable: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("health check returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func truncStr(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
