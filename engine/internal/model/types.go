package model

// Owner identifies the maintainer of a bundle.
type Owner struct {
	Name  string `yaml:"name" json:"name"`
	Email string `yaml:"email,omitempty" json:"email,omitempty"`
}

// SecretRef is the ONLY legal way to express a secret value in MCP env.
type SecretRef struct {
	SecretRef string `yaml:"secretRef" json:"secretRef"`
}

// Requirement is a presence+DAG dependency (no version ranges; see spec §7.3).
type Requirement struct {
	Type ArtifactType `yaml:"type" json:"type"`
	Ref  string       `yaml:"ref" json:"ref"` // "name" (same bundle) or "bundle/name"
}

// MCPConfig is the canonical MCP server definition (no executable body).
type MCPConfig struct {
	Transport string               `yaml:"transport" json:"transport"` // stdio | http
	Command   string               `yaml:"command,omitempty" json:"command,omitempty"`
	Args      []string             `yaml:"args,omitempty" json:"args,omitempty"`
	Env       map[string]SecretRef `yaml:"env,omitempty" json:"env,omitempty"`
	URL       string               `yaml:"url,omitempty" json:"url,omitempty"`
}

// Override is per-runtime AUTHOR INTENT only (not emulation scaffolding; spec §6.1).
type Override struct {
	// Fields holds frontmatter patches keyed by canonical field name.
	Fields map[string]any `yaml:",inline" json:"fields,omitempty"`
	// Body, if non-empty, fully replaces the body — requires DivergedReason.
	Body           string `yaml:"body,omitempty" json:"body,omitempty"`
	DivergedReason string `yaml:"-" json:"divergedReason,omitempty"` // parsed from `# diverged:` (Task in plan 02)
}

// Artifact is one canonical authored unit.
type Artifact struct {
	Name        string        `yaml:"name" json:"name"`
	Type        ArtifactType  `yaml:"type" json:"type"`
	Description string        `yaml:"description" json:"description"`
	Version     string        `yaml:"version" json:"version"`
	Tags        []string      `yaml:"tags,omitempty" json:"tags,omitempty"`
	Category    string        `yaml:"category,omitempty" json:"category,omitempty"`
	Maturity    Maturity      `yaml:"maturity,omitempty" json:"maturity,omitempty"`
	Summary     string        `yaml:"summary,omitempty" json:"summary,omitempty"`
	Runtimes    []Runtime     `yaml:"runtimes,omitempty" json:"runtimes,omitempty"`
	Requires    []Requirement `yaml:"requires,omitempty" json:"requires,omitempty"`

	// type-specific canonical fields
	ArgumentHint           string   `yaml:"argument-hint,omitempty" json:"argumentHint,omitempty"`
	Model                  string   `yaml:"model,omitempty" json:"model,omitempty"`
	DisableModelInvocation bool     `yaml:"disable-model-invocation,omitempty" json:"disableModelInvocation,omitempty"`
	AllowedTools           []string `yaml:"allowed-tools,omitempty" json:"allowedTools,omitempty"`
	Tools                  []string `yaml:"tools,omitempty" json:"tools,omitempty"` // agent

	MCP       *MCPConfig          `yaml:"mcp,omitempty" json:"mcp,omitempty"`
	Overrides map[Runtime]Override `yaml:"overrides,omitempty" json:"overrides,omitempty"`

	// runtime-populated
	Body       string `yaml:"-" json:"-"` // raw body incl. fences
	Bundle     string `yaml:"-" json:"-"` // owning bundle name
	SourcePath string `yaml:"-" json:"-"` // catalog-relative path

	// Raw is the frontmatter decoded for schema validation, run through a
	// YAML→JSON round-trip so number/string/bool fidelity matches jsonschema/v6
	// (which is JSON-typed: float64 for numbers, no yaml.Node, etc.). Not serialized.
	Raw any `yaml:"-" json:"-"`
}

// Bundle is the versioning + CC-plugin unit.
type Bundle struct {
	Name        string      `yaml:"name" json:"name"`
	Version     string      `yaml:"version" json:"version"`
	Description string      `yaml:"description" json:"description"`
	Category    string      `yaml:"category,omitempty" json:"category,omitempty"`
	Tags        []string    `yaml:"tags,omitempty" json:"tags,omitempty"`
	Owner       Owner       `yaml:"owner" json:"owner"`
	Maturity    Maturity    `yaml:"maturity,omitempty" json:"maturity,omitempty"`
	Runtimes    []Runtime   `yaml:"runtimes,omitempty" json:"runtimes,omitempty"`
	Homepage    string      `yaml:"homepage,omitempty" json:"homepage,omitempty"`
	Artifacts   []*Artifact `yaml:"-" json:"artifacts,omitempty"`
	SourcePath  string      `yaml:"-" json:"-"`
}

// Catalog is the whole loaded tree.
type Catalog struct {
	Bundles []*Bundle `json:"bundles"`
}
