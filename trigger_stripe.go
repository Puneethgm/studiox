package main

import (
	"encoding/json"
	"fmt"
	"github.com/stripe/stripe-go/v78"
)

func main() {
	payload := `{
  "type": "checkout.session.completed",
  "data": {
    "object": {
      "id": "cs_test_mock123",
      "object": "checkout.session",
      "amount_total": 2500,
      "currency": "sgd",
      "metadata": {
        "customer_phone": "+91 74839 74512",
        "customer_name": "john32ewsdx doe",
        "studio_id": "759b1ee2-5a68-4a5c-8fa0-5b2a64d5cc35"
      },
      "invoice": {
        "id": "in_12345",
        "hosted_invoice_url": "https://invoice.stripe.com/i/in_12345_mock_url"
      }
    }
  }
}`
	var event stripe.Event
	json.Unmarshal([]byte(payload), &event)
	fmt.Printf("Event Raw: %s\n", string(event.Data.Raw))
}
