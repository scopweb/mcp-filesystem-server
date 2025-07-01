package filesystemserver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// TaskPlan represents a planned task with steps
type TaskPlan struct {
	ID          string      `json:"id"`
	Description string      `json:"description"`
	Workspace   string      `json:"workspace"`
	Steps       []TaskStep  `json:"steps"`
	Complexity  string      `json:"complexity"`
	EstimatedOps int        `json:"estimated_ops"`
	RiskLevel   string      `json:"risk_level"`
	Dependencies []string   `json:"dependencies"`
}

// TaskStep represents a single step in the plan
type TaskStep struct {
	ID          int      `json:"id"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Files       []string `json:"files"`
	Command     string   `json:"command,omitempty"`
	Risk        string   `json:"risk"`
	Rollback    string   `json:"rollback"`
}

// handlePlanTask creates step-by-step execution plan for complex operations
func (fs *FilesystemHandler) handlePlanTask(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	description, _ := request.Params.Arguments["description"].(string)
	workspace, _ := request.Params.Arguments["workspace"].(string)
	targetFilesParam, _ := request.Params.Arguments["target_files"].([]interface{})

	if description == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: "❌ Error: description is required"},
			},
			IsError: true,
		}, nil
	}

	// Convert target files
	targetFiles := []string{}
	for _, file := range targetFilesParam {
		if str, ok := file.(string); ok {
			targetFiles = append(targetFiles, str)
		}
	}

	// Use current directory if no workspace specified
	if workspace == "" {
		cwd, err := os.Getwd()
		if err != nil {
			workspace = "."
		} else {
			workspace = cwd
		}
	}

	validWorkspace, err := fs.validatePath(workspace)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error: Invalid workspace: %v", err)},
			},
			IsError: true,
		}, nil
	}

	plan, err := fs.createTaskPlan(description, validWorkspace, targetFiles)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{Type: "text", Text: fmt.Sprintf("❌ Error creating plan: %v", err)},
			},
			IsError: true,
		}, nil
	}

	result := fs.formatTaskPlan(plan)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Type: "text", Text: result},
		},
	}, nil
}

// createTaskPlan analyzes the task and creates execution plan
func (fs *FilesystemHandler) createTaskPlan(description, workspace string, targetFiles []string) (*TaskPlan, error) {
	plan := &TaskPlan{
		ID:          generateTaskID(),
		Description: description,
		Workspace:   workspace,
		Steps:       []TaskStep{},
		Dependencies: []string{},
	}

	// Analyze workspace context
	context, err := fs.analyzeWorkspaceContext(workspace)
	if err != nil {
		return nil, err
	}

	// Generate steps based on task description
	steps := fs.generateStepsFromDescription(description, workspace, targetFiles, context)
	plan.Steps = steps

	// Calculate complexity and risk
	plan.Complexity = fs.calculateComplexityLevel(steps)
	plan.EstimatedOps = len(steps)
	plan.RiskLevel = fs.calculateRiskLevel(steps)
	plan.Dependencies = fs.extractTaskDependencies(steps, context)

	return plan, nil
}

// analyzeWorkspaceContext gathers project information
func (fs *FilesystemHandler) analyzeWorkspaceContext(workspace string) (map[string]interface{}, error) {
	context := make(map[string]interface{})

	// Detect project type
	projectType := fs.detectProjectType(workspace)
	context["project_type"] = projectType

	// Find important files
	importantFiles := fs.findImportantFiles(workspace)
	context["important_files"] = importantFiles

	// Get directory structure overview
	structure, _ := fs.getDirectoryOverview(workspace)
	context["structure"] = structure

	return context, nil
}

// detectProjectType identifies the type of project
func (fs *FilesystemHandler) detectProjectType(workspace string) string {
	patterns := map[string][]string{
		"go":         {"go.mod", "go.sum", "main.go"},
		"node":       {"package.json", "node_modules"},
		"python":     {"requirements.txt", "setup.py", "pyproject.toml"},
		"rust":       {"Cargo.toml", "Cargo.lock"},
		"java":       {"pom.xml", "build.gradle", "src/main/java"},
		"dotnet":     {"*.csproj", "*.sln", "Program.cs"},
		"web":        {"index.html", "src", "public"},
		"docker":     {"Dockerfile", "docker-compose.yml"},
	}

	for projectType, files := range patterns {
		for _, file := range files {
			if filepath.Ext(file) == "" {
				// Directory or exact file
				if _, err := os.Stat(filepath.Join(workspace, file)); err == nil {
					return projectType
				}
			} else {
				// Pattern matching
				matches, _ := filepath.Glob(filepath.Join(workspace, file))
				if len(matches) > 0 {
					return projectType
				}
			}
		}
	}

	return "unknown"
}

// findImportantFiles locates key configuration and source files
func (fs *FilesystemHandler) findImportantFiles(workspace string) []string {
	important := []string{}
	
	importantPatterns := []string{
		"*.go", "*.js", "*.ts", "*.py", "*.rs", "*.java", "*.cs",
		"package.json", "go.mod", "Cargo.toml", "requirements.txt",
		"Dockerfile", "docker-compose.yml", "Makefile",
		"README.md", "LICENSE", ".gitignore",
	}

	err := filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if _, err := fs.validatePath(path); err != nil {
			return nil
		}

		relPath, _ := filepath.Rel(workspace, path)
		filename := info.Name()

		for _, pattern := range importantPatterns {
			matched, _ := filepath.Match(pattern, filename)
			if matched {
				important = append(important, relPath)
				break
			}
		}

		// Limit to avoid huge lists
		if len(important) >= 50 {
			return filepath.SkipDir
		}

		return nil
	})

	if err == nil && len(important) > 20 {
		important = important[:20]
	}

	return important
}

// getDirectoryOverview provides high-level structure info
func (fs *FilesystemHandler) getDirectoryOverview(workspace string) (map[string]int, error) {
	overview := make(map[string]int)

	err := filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if _, err := fs.validatePath(path); err != nil {
			return nil
		}

		if info.IsDir() {
			overview["directories"]++
		} else {
			overview["files"]++
			ext := strings.ToLower(filepath.Ext(info.Name()))
			if ext != "" {
				overview[ext]++
			}
		}

		return nil
	})

	return overview, err
}

// generateStepsFromDescription creates steps based on task description
func (fs *FilesystemHandler) generateStepsFromDescription(description, workspace string, targetFiles []string, context map[string]interface{}) []TaskStep {
	steps := []TaskStep{}
	stepID := 1

	// Analyze description for key operations
	desc := strings.ToLower(description)

	// Backup step for risky operations
	if fs.isRiskyOperation(desc) {
		steps = append(steps, TaskStep{
			ID:          stepID,
			Type:        "backup",
			Description: "Create backup of files before modifications",
			Files:       targetFiles,
			Risk:        "low",
			Rollback:    "Restore from backup",
		})
		stepID++
	}

	// Add specific steps based on keywords
	if strings.Contains(desc, "refactor") || strings.Contains(desc, "restructure") {
		steps = append(steps, fs.generateRefactorSteps(targetFiles, stepID)...)
		stepID += len(steps)
	}

	if strings.Contains(desc, "move") || strings.Contains(desc, "rename") {
		steps = append(steps, fs.generateMoveSteps(targetFiles, stepID)...)
		stepID += len(steps)
	}

	if strings.Contains(desc, "add") || strings.Contains(desc, "create") {
		steps = append(steps, fs.generateCreateSteps(description, workspace, stepID)...)
		stepID += len(steps)
	}

	if strings.Contains(desc, "delete") || strings.Contains(desc, "remove") {
		steps = append(steps, fs.generateDeleteSteps(targetFiles, stepID)...)
		stepID += len(steps)
	}

	// Validation step
	if len(steps) > 1 {
		steps = append(steps, TaskStep{
			ID:          stepID,
			Type:        "validate",
			Description: "Validate changes and run basic checks",
			Files:       targetFiles,
			Risk:        "low",
			Rollback:    "Fix validation errors",
		})
	}

	// Default fallback step
	if len(steps) == 0 {
		steps = append(steps, TaskStep{
			ID:          1,
			Type:        "analyze",
			Description: "Analyze requirements and plan detailed approach",
			Files:       targetFiles,
			Risk:        "low",
			Rollback:    "No changes made",
		})
	}

	return steps
}

// Helper functions for step generation
func (fs *FilesystemHandler) generateRefactorSteps(files []string, startID int) []TaskStep {
	return []TaskStep{
		{
			ID:          startID,
			Type:        "analyze",
			Description: "Analyze code dependencies and relationships",
			Files:       files,
			Risk:        "low",
			Rollback:    "No changes made",
		},
		{
			ID:          startID + 1,
			Type:        "modify",
			Description: "Apply refactoring changes incrementally",
			Files:       files,
			Risk:        "medium",
			Rollback:    "Revert file changes",
		},
	}
}

func (fs *FilesystemHandler) generateMoveSteps(files []string, startID int) []TaskStep {
	return []TaskStep{
		{
			ID:          startID,
			Type:        "copy",
			Description: "Copy files to new location",
			Files:       files,
			Risk:        "low",
			Rollback:    "Delete copied files",
		},
		{
			ID:          startID + 1,
			Type:        "update",
			Description: "Update references and imports",
			Files:       []string{"*"},
			Risk:        "medium",
			Rollback:    "Restore original references",
		},
		{
			ID:          startID + 2,
			Type:        "delete",
			Description: "Remove original files",
			Files:       files,
			Risk:        "high",
			Rollback:    "Restore from backup",
		},
	}
}

func (fs *FilesystemHandler) generateCreateSteps(description, workspace string, startID int) []TaskStep {
	return []TaskStep{
		{
			ID:          startID,
			Type:        "create",
			Description: "Create new files/directories",
			Files:       []string{"new files"},
			Risk:        "low",
			Rollback:    "Delete created files",
		},
	}
}

func (fs *FilesystemHandler) generateDeleteSteps(files []string, startID int) []TaskStep {
	return []TaskStep{
		{
			ID:          startID,
			Type:        "delete",
			Description: "Remove specified files/directories",
			Files:       files,
			Risk:        "high",
			Rollback:    "Restore from backup",
		},
	}
}

// Risk and complexity calculation
func (fs *FilesystemHandler) isRiskyOperation(description string) bool {
	riskyKeywords := []string{"delete", "remove", "move", "refactor", "restructure", "migrate"}
	for _, keyword := range riskyKeywords {
		if strings.Contains(description, keyword) {
			return true
		}
	}
	return false
}

func (fs *FilesystemHandler) calculateComplexityLevel(steps []TaskStep) string {
	if len(steps) <= 2 {
		return "low"
	} else if len(steps) <= 5 {
		return "medium"
	}
	return "high"
}

func (fs *FilesystemHandler) calculateRiskLevel(steps []TaskStep) string {
	hasHighRisk := false
	hasMediumRisk := false

	for _, step := range steps {
		switch step.Risk {
		case "high":
			hasHighRisk = true
		case "medium":
			hasMediumRisk = true
		}
	}

	if hasHighRisk {
		return "high"
	} else if hasMediumRisk {
		return "medium"
	}
	return "low"
}

func (fs *FilesystemHandler) extractTaskDependencies(steps []TaskStep, context map[string]interface{}) []string {
	deps := []string{}
	
	// Add project-specific dependencies
	if projectType, ok := context["project_type"].(string); ok {
		switch projectType {
		case "go":
			deps = append(deps, "go compiler", "go.mod")
		case "node":
			deps = append(deps, "node.js", "npm/yarn")
		case "python":
			deps = append(deps, "python interpreter", "pip")
		}
	}

	return deps
}

// formatTaskPlan formats the plan for display
func (fs *FilesystemHandler) formatTaskPlan(plan *TaskPlan) string {
	var result strings.Builder

	result.WriteString("📋 **Task Execution Plan**\n\n")
	result.WriteString(fmt.Sprintf("**ID:** %s\n", plan.ID))
	result.WriteString(fmt.Sprintf("**Description:** %s\n", plan.Description))
	result.WriteString(fmt.Sprintf("**Workspace:** %s\n", plan.Workspace))
	result.WriteString(fmt.Sprintf("**Complexity:** %s | **Risk:** %s | **Operations:** %d\n\n", 
		plan.Complexity, plan.RiskLevel, plan.EstimatedOps))

	if len(plan.Dependencies) > 0 {
		result.WriteString("**Dependencies:**\n")
		for _, dep := range plan.Dependencies {
			result.WriteString(fmt.Sprintf("  • %s\n", dep))
		}
		result.WriteString("\n")
	}

	result.WriteString("**Execution Steps:**\n")
	for _, step := range plan.Steps {
		riskEmoji := "🟢"
		if step.Risk == "medium" {
			riskEmoji = "🟡"
		} else if step.Risk == "high" {
			riskEmoji = "🔴"
		}

		result.WriteString(fmt.Sprintf("%d. %s **%s** - %s\n", 
			step.ID, riskEmoji, strings.ToUpper(step.Type), step.Description))
		
		if len(step.Files) > 0 && step.Files[0] != "*" && step.Files[0] != "new files" {
			result.WriteString(fmt.Sprintf("   📁 Files: %s\n", strings.Join(step.Files, ", ")))
		}
		
		result.WriteString(fmt.Sprintf("   🔄 Rollback: %s\n", step.Rollback))
		result.WriteString("\n")
	}

	result.WriteString("💡 **Recommendation:** Review each step before execution. Create checkpoints for high-risk operations.\n")

	return result.String()
}

// generateTaskID creates unique task identifier
func generateTaskID() string {
	return fmt.Sprintf("task_%d", 1000+len(fmt.Sprintf("%d", 12345))) // Simple ID generation
}
