package scenario

import (
	"encoding/json"
	"fmt"
	"os"
)

func LoadFile(path string) (Fixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Fixture{}, err
	}
	var fixture Fixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		return Fixture{}, err
	}
	if fixture.Name == "" {
		return Fixture{}, fmt.Errorf("fixture name is required")
	}
	if len(fixture.Actions) == 0 {
		return Fixture{}, fmt.Errorf("fixture %s has no actions", fixture.Name)
	}
	return fixture, nil
}
