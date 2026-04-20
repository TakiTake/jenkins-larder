package unit

import (
	"testing"

	"github.com/yourorg/jenkins-larder/src/upstream"
)

func TestPluginURL(t *testing.T) {
	client := upstream.NewClient("https://updates.jenkins.io", 60)

	tests := []struct {
		name, version, ext, want string
	}{
		{"git", "4.11.0", "hpi", "https://updates.jenkins.io/download/plugins/git/4.11.0/git.hpi"},
		{"credentials", "2.6.1", "jpi", "https://updates.jenkins.io/download/plugins/credentials/2.6.1/credentials.jpi"},
		{"workflow-aggregator", "596.v8c21c963d92d", "hpi", "https://updates.jenkins.io/download/plugins/workflow-aggregator/596.v8c21c963d92d/workflow-aggregator.hpi"},
	}

	for _, tt := range tests {
		t.Run(tt.name+"/"+tt.version, func(t *testing.T) {
			got := client.PluginURL(tt.name, tt.version, tt.ext)
			if got != tt.want {
				t.Errorf("PluginURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUpdateCenterURL(t *testing.T) {
	client := upstream.NewClient("https://updates.jenkins.io", 60)
	got := client.UpdateCenterURL()
	want := "https://updates.jenkins.io/update-center.json"
	if got != want {
		t.Errorf("UpdateCenterURL() = %q, want %q", got, want)
	}
}

func TestParsePluginURL(t *testing.T) {
	tests := []struct {
		path                          string
		wantName, wantVersion, wantExt string
		wantErr                       bool
	}{
		{
			path:        "/download/plugins/git/4.11.0/git.hpi",
			wantName:    "git",
			wantVersion: "4.11.0",
			wantExt:     "hpi",
		},
		{
			path:        "/download/plugins/credentials/2.6.1/credentials.jpi",
			wantName:    "credentials",
			wantVersion: "2.6.1",
			wantExt:     "jpi",
		},
		{
			path:        "/download/plugins/workflow-aggregator/596.v8c21c963d92d/workflow-aggregator.hpi",
			wantName:    "workflow-aggregator",
			wantVersion: "596.v8c21c963d92d",
			wantExt:     "hpi",
		},
		{
			path:    "/download/plugins/git/4.11.0/wrong.hpi",
			wantErr: true,
		},
		{
			path:    "/download/plugins/git/4.11.0/git",
			wantErr: true,
		},
		{
			path:    "/other/path",
			wantErr: true,
		},
		{
			path:    "/download/plugins/git",
			wantErr: true,
		},
		{
			path:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			name, version, ext, err := upstream.ParsePluginURL(tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParsePluginURL(%q) error = %v, wantErr = %v", tt.path, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if version != tt.wantVersion {
				t.Errorf("version = %q, want %q", version, tt.wantVersion)
			}
			if ext != tt.wantExt {
				t.Errorf("ext = %q, want %q", ext, tt.wantExt)
			}
		})
	}
}
