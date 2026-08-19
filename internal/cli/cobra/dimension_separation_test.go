package cobra

import "testing"

func TestScientificCommandsDoNotExposeKochGeneratorFlags(t *testing.T) {
	syntheticFlags := []string{"iterations", "seed", "angle-jitter", "height-jitter", "model-max-points", "no-model-simplify"}
	for _, flag := range syntheticFlags {
		if dimensionCmd.Flags().Lookup(flag) != nil {
			t.Errorf("флаг %q не должен присутствовать у научной команды dimension", flag)
		}
		if kochDemoCmd.Flags().Lookup(flag) == nil {
			t.Errorf("флаг %q должен быть доступен у учебной команды koch-demo", flag)
		}
	}
	for _, flag := range []string{"iterations", "seed", "angle-jitter", "height-jitter"} {
		if allCmd.Flags().Lookup(flag) != nil {
			t.Errorf("флаг генератора %q не должен присутствовать у научного конвейера all", flag)
		}
	}
}
