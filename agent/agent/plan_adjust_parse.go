package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// planAdjustStepFlex 兼容模型返回的 description 等别名字段。
type planAdjustStepFlex struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Instruction     string   `json:"instruction"`
	Description     string   `json:"description"`
	Text            string   `json:"text"`
	CapabilityHints []string `json:"capability_hints"`
	Tier            int      `json:"tier"`
}

// parsePlanAdjustJSON 宽容解析调节 JSON：支持 new_steps 为对象数组、字符串数组、或仅含 description 的对象。
func parsePlanAdjustJSON(raw string) (planAdjustJSON, error) {
	raw = extractJSONObject(strings.TrimSpace(raw))
	var flex struct {
		Action   string          `json:"action"`
		Reason   string          `json:"reason"`
		NewSteps json.RawMessage `json:"new_steps"`
	}
	if err := json.Unmarshal([]byte(raw), &flex); err != nil {
		return planAdjustJSON{}, fmt.Errorf("解析调节 JSON: %w", err)
	}
	adj := planAdjustJSON{
		Action: strings.TrimSpace(flex.Action),
		Reason: strings.TrimSpace(flex.Reason),
	}
	if len(flex.NewSteps) == 0 || strings.TrimSpace(string(flex.NewSteps)) == "null" {
		return adj, nil
	}
	steps, err := parsePlanAdjustNewSteps(flex.NewSteps)
	if err != nil {
		return planAdjustJSON{}, err
	}
	adj.NewSteps = steps
	return adj, nil
}

func parsePlanAdjustNewSteps(raw json.RawMessage) ([]planStepJSON, error) {
	var flexSteps []planAdjustStepFlex
	if err := json.Unmarshal(raw, &flexSteps); err == nil && len(flexSteps) > 0 {
		out := make([]planStepJSON, 0, len(flexSteps))
		for _, fs := range flexSteps {
			out = append(out, flexStepToPlanStep(fs))
		}
		return out, nil
	}
	var strs []string
	if err := json.Unmarshal(raw, &strs); err == nil && len(strs) > 0 {
		out := make([]planStepJSON, 0, len(strs))
		for _, s := range strs {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			out = append(out, planStepJSON{Instruction: s})
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	var steps []planStepJSON
	if err := json.Unmarshal(raw, &steps); err == nil && len(steps) > 0 {
		out := make([]planStepJSON, 0, len(steps))
		for _, s := range steps {
			if step := coalescePlanStepJSON(s); step.Instruction != "" || step.Title != "" {
				out = append(out, step)
			}
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	return nil, fmt.Errorf("new_steps 格式无法识别")
}

func flexStepToPlanStep(fs planAdjustStepFlex) planStepJSON {
	instr := strings.TrimSpace(fs.Instruction)
	if instr == "" {
		instr = strings.TrimSpace(fs.Description)
	}
	if instr == "" {
		instr = strings.TrimSpace(fs.Text)
	}
	title := strings.TrimSpace(fs.Title)
	return planStepJSON{
		ID:              strings.TrimSpace(fs.ID),
		Title:           title,
		Instruction:     instr,
		CapabilityHints: fs.CapabilityHints,
		Tier:            fs.Tier,
	}
}

func coalescePlanStepJSON(s planStepJSON) planStepJSON {
	s.ID = strings.TrimSpace(s.ID)
	s.Title = strings.TrimSpace(s.Title)
	s.Instruction = strings.TrimSpace(s.Instruction)
	return s
}
