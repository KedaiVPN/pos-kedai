package utils

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

type TripayClient struct {
	MerchantCode string
	ApiKey       string
	PrivateKey   string
	IsSandbox    bool
	BaseURL      string
}

type TripayFee struct {
	Flat    int         `json:"flat"`
	Percent interface{} `json:"percent"`
}

type TripayPaymentChannel struct {
	Group       string    `json:"group"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	FeeMerchant TripayFee `json:"fee_merchant"`
	FeeCustomer TripayFee `json:"fee_customer"`
	TotalFee    TripayFee `json:"total_fee"`
	IconURL     string    `json:"icon_url"`
	Active      bool      `json:"active"`
}

type TripayInstruction struct {
	Title string   `json:"title"`
	Steps []string `json:"steps"`
}

type TripayTransactionItem struct {
	SKU      string `json:"sku"`
	Name     string `json:"name"`
	Price    int64  `json:"price"`
	Quantity int    `json:"quantity"`
}

type TripayCreateTransactionRequest struct {
	Method        string                  `json:"method"`
	MerchantRef   string                  `json:"merchant_ref"`
	Amount        int64                   `json:"amount"`
	CustomerName  string                  `json:"customer_name"`
	CustomerEmail string                  `json:"customer_email"`
	CustomerPhone string                  `json:"customer_phone,omitempty"`
	OrderItems    []TripayTransactionItem `json:"order_items"`
	Signature     string                  `json:"signature"`
	Expiry        int64                   `json:"expiry,omitempty"`
}

type TripayTransactionResponseData struct {
	Reference      string              `json:"reference"`
	MerchantRef    string              `json:"merchant_ref"`
	PaymentMethod  string              `json:"payment_method"`
	PaymentName    string              `json:"payment_name"`
	CustomerName   string              `json:"customer_name"`
	Amount         int64               `json:"amount"`
	FeeMerchant    int64               `json:"fee_merchant"`
	FeeCustomer    int64               `json:"fee_customer"`
	TotalFee       int64               `json:"total_fee"`
	AmountReceived int64               `json:"amount_received"`
	PayCode        string              `json:"pay_code"`
	QrString       string              `json:"qr_string"`
	QrURL          string              `json:"qr_url"`
	CheckoutURL    string              `json:"checkout_url"`
	Status         string              `json:"status"`
	PaidAt         *int64              `json:"paid_at"`
	ExpiredTime    int64               `json:"expired_time"`
	Instructions   []TripayInstruction `json:"instructions"`
}

type TripayFeeDetail struct {
	Flat    int64       `json:"flat"`
	Percent interface{} `json:"percent"`
}

type TripayTotalFeeDetail struct {
	Merchant int64 `json:"merchant"`
	Customer int64 `json:"customer"`
}

type TripayFeeCalculatorData struct {
	Code     string               `json:"code"`
	Name     string               `json:"name"`
	Fee      TripayFeeDetail      `json:"fee"`
	TotalFee TripayTotalFeeDetail `json:"total_fee"`
}

type TripayResponse[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func NewTripayClient() *TripayClient {
	merchantCode := os.Getenv("TRIPAY_MERCHANT_CODE")
	apiKey := os.Getenv("TRIPAY_API_KEY")
	privateKey := os.Getenv("TRIPAY_PRIVATE_KEY")

	baseURL := "https://tripay.co.id/api-sandbox"
	if os.Getenv("TRIPAY_MODE") == "production" {
		baseURL = "https://tripay.co.id/api"
	}

	return &TripayClient{
		MerchantCode: merchantCode,
		ApiKey:       apiKey,
		PrivateKey:   privateKey,
		IsSandbox:    os.Getenv("TRIPAY_MODE") != "production",
		BaseURL:      baseURL,
	}
}

func (c *TripayClient) GenerateSignature(merchantRef string, amount int64) string {
	data := fmt.Sprintf("%s%s%d", c.MerchantCode, merchantRef, amount)
	h := hmac.New(sha256.New, []byte(c.PrivateKey))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func (c *TripayClient) VerifyCallbackSignature(rawBody []byte, callbackSignature string) bool {
	h := hmac.New(sha256.New, []byte(c.PrivateKey))
	h.Write(rawBody)
	expectedSignature := hex.EncodeToString(h.Sum(nil))
	return hmac.Equal([]byte(expectedSignature), []byte(callbackSignature))
}

func (c *TripayClient) GetPaymentChannels() ([]TripayPaymentChannel, error) {
	req, err := http.NewRequest("GET", c.BaseURL+"/merchant/payment-channel", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.ApiKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result TripayResponse[[]TripayPaymentChannel]
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal channels: %v, raw: %s", err, string(bodyBytes))
	}

	if !result.Success {
		return nil, fmt.Errorf("tripay error: %s", result.Message)
	}

	return result.Data, nil
}

func (c *TripayClient) GetFeeCalculator(code string, amount int64) ([]TripayFeeCalculatorData, error) {
	reqURL := fmt.Sprintf("%s/merchant/fee-calculator?code=%s&amount=%d", c.BaseURL, url.QueryEscape(code), amount)

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.ApiKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result TripayResponse[[]TripayFeeCalculatorData]
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal fee calculator response: %v, raw: %s", err, string(bodyBytes))
	}

	if !result.Success {
		return nil, fmt.Errorf("tripay error: %s", result.Message)
	}

	return result.Data, nil
}

func (c *TripayClient) CreateTransaction(reqPayload TripayCreateTransactionRequest) (*TripayTransactionResponseData, error) {
	reqPayload.Signature = c.GenerateSignature(reqPayload.MerchantRef, reqPayload.Amount)

	jsonBody, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.BaseURL+"/transaction/create", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result TripayResponse[TripayTransactionResponseData]
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal transaction create response: %v, raw: %s", err, string(bodyBytes))
	}

	if !result.Success {
		return nil, fmt.Errorf("tripay error: %s", result.Message)
	}

	return &result.Data, nil
}

func (c *TripayClient) GetPaymentInstructions(code string, payCode string, amount int64) ([]TripayInstruction, error) {
	reqURL := fmt.Sprintf("%s/payment/instruction?code=%s&allow_html=1", c.BaseURL, url.QueryEscape(code))
	if payCode != "" {
		reqURL += "&pay_code=" + url.QueryEscape(payCode)
	}
	if amount > 0 {
		reqURL += fmt.Sprintf("&amount=%d", amount)
	}

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.ApiKey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result TripayResponse[[]TripayInstruction]
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal instructions: %v, raw: %s", err, string(bodyBytes))
	}

	if !result.Success {
		return nil, fmt.Errorf("tripay error: %s", result.Message)
	}

	return result.Data, nil
}
