package harvester

import (
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestSetLogFormat(t *testing.T) {
	t.Cleanup(func() { log.SetFormatter(&log.TextFormatter{DisableTimestamp: true}) })

	t.Run("json", func(t *testing.T) {
		setLogFormat("json")

		if _, ok := log.StandardLogger().Formatter.(*log.JSONFormatter); !ok {
			t.Errorf("formatter = %T, want *logrus.JSONFormatter", log.StandardLogger().Formatter)
		}
	})

	t.Run("anything else defaults to text", func(t *testing.T) {
		for _, v := range []string{"text", "", "bogus"} {
			setLogFormat(v)

			if _, ok := log.StandardLogger().Formatter.(*log.TextFormatter); !ok {
				t.Errorf("setLogFormat(%q): formatter = %T, want *logrus.TextFormatter", v, log.StandardLogger().Formatter)
			}
		}
	})
}

func TestSetLogLevel(t *testing.T) {
	t.Cleanup(func() { log.SetLevel(log.DebugLevel) })

	setLogLevel("info")

	if log.GetLevel() != log.InfoLevel {
		t.Errorf("level = %v, want %v", log.GetLevel(), log.InfoLevel)
	}

	setLogLevel("not-a-level")

	if log.GetLevel() != log.WarnLevel {
		t.Errorf("setLogLevel(invalid): level = %v, want %v (the safe fallback)", log.GetLevel(), log.WarnLevel)
	}
}
