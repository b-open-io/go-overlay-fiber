package server

import (
	"bytes"
	"fmt"
	"html/template"
	"path/filepath"
	"runtime"
	"sync"
)

// TemplateManager handles HTML template loading and rendering
type TemplateManager struct {
	templates map[string]*template.Template
	mutex     sync.RWMutex
}

// NewTemplateManager creates a new template manager
func NewTemplateManager() *TemplateManager {
	return &TemplateManager{
		templates: make(map[string]*template.Template),
	}
}

// LoadTemplates loads all HTML templates from the templates directory
func (tm *TemplateManager) LoadTemplates() error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	// Get the directory where this source file is located
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("failed to get current file path")
	}

	// Build path to templates directory
	templatesDir := filepath.Join(filepath.Dir(filename), "templates")

	// Define template files to load
	templateFiles := map[string]string{
		"main":                "main.html",
		"dashboard":           "dashboard.html",
		"dynamic-interface":   "dynamic-interface.html",
		"api-tester":          "api-tester.html",
		"topic-managers":      "topic-managers.html",
		"lookup-services":     "lookup-services.html",
		"topic-manager-docs":  "topic-manager-docs.html",
		"lookup-service-docs": "lookup-service-docs.html",
	}

	// Load each template
	for name, filename := range templateFiles {
		templatePath := filepath.Join(templatesDir, filename)
		tmpl, err := template.ParseFiles(templatePath)
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %w", name, err)
		}
		tm.templates[name] = tmpl
	}

	return nil
}

// RenderTemplate renders a template with the given data
func (tm *TemplateManager) RenderTemplate(name string, data interface{}) (string, error) {
	tm.mutex.RLock()
	tmpl, exists := tm.templates[name]
	tm.mutex.RUnlock()

	if !exists {
		return "", fmt.Errorf("template %s not found", name)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", name, err)
	}

	return buf.String(), nil
}

// MainPageData represents data for the main page template
type MainPageData struct {
	Name           string
	Network        string
	DatabaseStatus string
	EngineStatus   string
	StatusClass    string
	StatusText     string
	StorageType    string
}

// DashboardData represents data for the enhanced dashboard template
type DashboardData struct {
	Name                 string
	Network              string
	FQDN                 string
	Port                 int
	Timestamp            string
	AdminTokenConfigured bool
	GASPSyncEnabled      bool

	// Component status
	QueueManager     map[string]interface{}
	WebSocketManager map[string]interface{}
	Databases        map[string]interface{}
	Engine           map[string]interface{}

	// Storage information
	StorageType   string
	StorageStatus string
	StorageConfig map[string]interface{}
}

// DynamicInterfaceData represents data for the dynamic interface template
type DynamicInterfaceData struct {
	Name         string
	Network      string
	FQDN         string
	StorageInfo  string
	BaseURL      string
	WebSocketURL string
}

// APITesterData represents data for the API tester template
type APITesterData struct {
	Name         string
	WebSocketURL string
}
