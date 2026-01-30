package config

import "fmt"

func (c *Config) Validate() error {
	if len(c.Sources) == 0 {
		return fmt.Errorf("at least one source is required")
	}

	if len(c.Sinks) == 0 {
		return fmt.Errorf("at least one sink is required")
	}

	for name, t := range c.Transforms {
		if len(t.Inputs) == 0 {
			return fmt.Errorf("transform [%s]: inputs list is empty", name)
		}
		for _, inputName := range t.Inputs {
			if !c.componentExists(inputName) {
				return fmt.Errorf("transform [%s]: refers to unknown input '%s'", name, inputName)
			}
		}
	}

	for name, s := range c.Sinks {
		if len(s.Inputs) == 0 {
			return fmt.Errorf("sink [%s]: inputs list is empty", name)
		}
		for _, inputName := range s.Inputs {
			if !c.componentExists(inputName) {
				return fmt.Errorf("sink [%s]: refers to unknown input '%s'", name, inputName)
			}
		}
	}

	return nil
}

func (c *Config) componentExists(name string) bool {
	_, existsInSources := c.Sources[name]
	_, existsInTransforms := c.Transforms[name]
	return existsInSources || existsInTransforms
}