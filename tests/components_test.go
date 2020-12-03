package main

import (
	"encoding/json"
	"testing"

	"github.com/arkjxu/loafer"
)

func TestModal(t *testing.T) {
	validView := `{"type":"modal","title":{"type":"plain_text","text":"Test Modal"},"submit":{"type":"plain_text","text":"Submit"},"close":{"type":"plain_text","text":"Cancel"},"blocks":[{"type":"context","elements":[{"type":"plain_text","text":"hello"}]},{"type":"input","element":{"type":"timepicker","action_id":"test_picker"},"label":{"type":"plain_text","text":"Test picker","emoji":true}}],"callback_id":"test_callback"}`
	blocks := []interface{}{}
	blocks = append(blocks, loafer.MakeSlackContext("hello"))
	blocks = append(blocks, loafer.MakeSlackModalTimePickerInput("Test picker", "Please pick a time", "", "test_picker"))
	view := loafer.MakeSlackModal("Test Modal", "test_callback", blocks, "Submit", "Cancel", false)
	jsonView, err := json.Marshal(view)
	if err != nil {
		t.Errorf("%v", err)
	}
	if string(jsonView) != validView {
		t.Errorf("%s", "Slack UI Generation failed, not valid")
	}
}
