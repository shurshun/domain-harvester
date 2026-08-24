package harvester

import (
	"reflect"
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

func TestParseSourcePriority(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"typical flag value", "cluster,config,ingressroute", []string{"cluster", "config", "ingressroute"}},
		{"spaces around entries are trimmed", "cluster, config , ingressroute", []string{"cluster", "config", "ingressroute"}},
		{"trailing comma doesn't add an empty entry", "cluster,config,", []string{"cluster", "config"}},
		{"empty string yields nil, not [\"\"]", "", nil},
		{"only commas yields nil", ",,", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseSourcePriority(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseSourcePriority(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
