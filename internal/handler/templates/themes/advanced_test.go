package themes

import (
	"reflect"
	"testing"
)

func TestThemeData_Structure(t *testing.T) {
	data := ThemeData{
		Path:        "/test/path",
		Parent:      true,
		Files:       []FileItem{},
		FileCount:   10,
		Breadcrumbs: []BreadcrumbItem{},
		CSS:         "/* test css */",
		JS:          "// test js",
	}

	// Test field access
	if data.Path != "/test/path" {
		t.Errorf("Expected Path '/test/path', got %q", data.Path)
	}

	if !data.Parent {
		t.Error("Expected Parent to be true")
	}

	if data.FileCount != 10 {
		t.Errorf("Expected FileCount 10, got %d", data.FileCount)
	}

	if data.CSS != "/* test css */" {
		t.Errorf("Expected CSS '/* test css */', got %q", data.CSS)
	}

	if data.JS != "// test js" {
		t.Errorf("Expected JS '// test js', got %q", data.JS)
	}
}

func TestThemeData_EmptyFiles(t *testing.T) {
	data := ThemeData{
		Files: []FileItem{},
	}

	if data.Files == nil {
		t.Error("Files should be initialized as empty slice, not nil")
	}

	if len(data.Files) != 0 {
		t.Errorf("Expected empty Files slice, got length %d", len(data.Files))
	}
}

func TestThemeData_EmptyBreadcrumbs(t *testing.T) {
	data := ThemeData{
		Breadcrumbs: []BreadcrumbItem{},
	}

	if data.Breadcrumbs == nil {
		t.Error("Breadcrumbs should be initialized as empty slice, not nil")
	}

	if len(data.Breadcrumbs) != 0 {
		t.Errorf("Expected empty Breadcrumbs slice, got length %d", len(data.Breadcrumbs))
	}
}

func TestFileItem_Structure(t *testing.T) {
	file := FileItem{
		Name:          "test.txt",
		IsDir:         false,
		Size:          1024,
		FormattedSize: "1.0 KB",
		FormattedTime: "2023-01-01 12:00:00",
	}

	// Test field access
	if file.Name != "test.txt" {
		t.Errorf("Expected Name 'test.txt', got %q", file.Name)
	}

	if file.IsDir {
		t.Error("Expected IsDir to be false")
	}

	if file.Size != 1024 {
		t.Errorf("Expected Size 1024, got %d", file.Size)
	}

	if file.FormattedSize != "1.0 KB" {
		t.Errorf("Expected FormattedSize '1.0 KB', got %q", file.FormattedSize)
	}

	if file.FormattedTime != "2023-01-01 12:00:00" {
		t.Errorf("Expected FormattedTime '2023-01-01 12:00:00', got %q", file.FormattedTime)
	}
}

func TestFileItem_Directory(t *testing.T) {
	dir := FileItem{
		Name:          "documents",
		IsDir:         true,
		Size:          0,
		FormattedSize: "-",
		FormattedTime: "2023-01-01 12:00:00",
	}

	if !dir.IsDir {
		t.Error("Expected IsDir to be true for directory")
	}

	if dir.Size != 0 {
		t.Errorf("Expected Size 0 for directory, got %d", dir.Size)
	}
}

func TestBreadcrumbItem_Structure(t *testing.T) {
	crumb := BreadcrumbItem{
		Name: "Documents",
		Path: "/home/user/documents",
	}

	if crumb.Name != "Documents" {
		t.Errorf("Expected Name 'Documents', got %q", crumb.Name)
	}

	if crumb.Path != "/home/user/documents" {
		t.Errorf("Expected Path '/home/user/documents', got %q", crumb.Path)
	}
}

func TestBreadcrumbItem_Root(t *testing.T) {
	root := BreadcrumbItem{
		Name: "Home",
		Path: "/",
	}

	if root.Name != "Home" {
		t.Errorf("Expected Name 'Home', got %q", root.Name)
	}

	if root.Path != "/" {
		t.Errorf("Expected Path '/', got %q", root.Path)
	}
}

func TestThemeData_WithFiles(t *testing.T) {
	files := []FileItem{
		{
			Name:          "readme.txt",
			IsDir:         false,
			Size:          256,
			FormattedSize: "256 B",
			FormattedTime: "2023-01-01 10:00:00",
		},
		{
			Name:          "documents",
			IsDir:         true,
			Size:          0,
			FormattedSize: "-",
			FormattedTime: "2023-01-01 11:00:00",
		},
	}

	data := ThemeData{
		Path:      "/test",
		Files:     files,
		FileCount: len(files),
	}

	if len(data.Files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(data.Files))
	}

	if data.FileCount != 2 {
		t.Errorf("Expected FileCount 2, got %d", data.FileCount)
	}

	// Test first file
	if data.Files[0].Name != "readme.txt" {
		t.Errorf("Expected first file name 'readme.txt', got %q", data.Files[0].Name)
	}

	if data.Files[0].IsDir {
		t.Error("Expected first file to not be directory")
	}

	// Test second file (directory)
	if data.Files[1].Name != "documents" {
		t.Errorf("Expected second file name 'documents', got %q", data.Files[1].Name)
	}

	if !data.Files[1].IsDir {
		t.Error("Expected second file to be directory")
	}
}

func TestThemeData_WithBreadcrumbs(t *testing.T) {
	breadcrumbs := []BreadcrumbItem{
		{Name: "Home", Path: "/"},
		{Name: "Documents", Path: "/documents"},
		{Name: "Projects", Path: "/documents/projects"},
	}

	data := ThemeData{
		Path:        "/documents/projects",
		Breadcrumbs: breadcrumbs,
	}

	if len(data.Breadcrumbs) != 3 {
		t.Errorf("Expected 3 breadcrumbs, got %d", len(data.Breadcrumbs))
	}

	// Test breadcrumb navigation
	expectedPaths := []string{"/", "/documents", "/documents/projects"}
	expectedNames := []string{"Home", "Documents", "Projects"}

	for i, crumb := range data.Breadcrumbs {
		if crumb.Path != expectedPaths[i] {
			t.Errorf("Breadcrumb %d: expected path %q, got %q", i, expectedPaths[i], crumb.Path)
		}
		if crumb.Name != expectedNames[i] {
			t.Errorf("Breadcrumb %d: expected name %q, got %q", i, expectedNames[i], crumb.Name)
		}
	}
}

func TestAdvancedEmbedded_Content(t *testing.T) {
	// Test that embedded content is available
	if AdvancedHTML == "" {
		t.Error("AdvancedHTML should not be empty")
	}

	if AdvancedCSS == "" {
		t.Error("AdvancedCSS should not be empty")
	}

	if AdvancedJS == "" {
		t.Error("AdvancedJS should not be empty")
	}

	// Test content length (should be substantial)
	if len(AdvancedHTML) < 100 {
		t.Errorf("AdvancedHTML seems too short: %d characters", len(AdvancedHTML))
	}

	if len(AdvancedCSS) < 100 {
		t.Errorf("AdvancedCSS seems too short: %d characters", len(AdvancedCSS))
	}

	if len(AdvancedJS) < 100 {
		t.Errorf("AdvancedJS seems too short: %d characters", len(AdvancedJS))
	}
}

// Test zero values and edge cases
func TestThemeData_ZeroValues(t *testing.T) {
	var data ThemeData

	if data.Path != "" {
		t.Errorf("Expected empty Path, got %q", data.Path)
	}

	if data.Parent {
		t.Error("Expected Parent to be false by default")
	}

	if data.FileCount != 0 {
		t.Errorf("Expected FileCount 0, got %d", data.FileCount)
	}

	if data.Files != nil {
		t.Error("Expected Files to be nil by default")
	}

	if data.Breadcrumbs != nil {
		t.Error("Expected Breadcrumbs to be nil by default")
	}
}

func TestFileItem_ZeroValues(t *testing.T) {
	var file FileItem

	if file.Name != "" {
		t.Errorf("Expected empty Name, got %q", file.Name)
	}

	if file.IsDir {
		t.Error("Expected IsDir to be false by default")
	}

	if file.Size != 0 {
		t.Errorf("Expected Size 0, got %d", file.Size)
	}

	if file.FormattedSize != "" {
		t.Errorf("Expected empty FormattedSize, got %q", file.FormattedSize)
	}

	if file.FormattedTime != "" {
		t.Errorf("Expected empty FormattedTime, got %q", file.FormattedTime)
	}
}

func TestBreadcrumbItem_ZeroValues(t *testing.T) {
	var crumb BreadcrumbItem

	if crumb.Name != "" {
		t.Errorf("Expected empty Name, got %q", crumb.Name)
	}

	if crumb.Path != "" {
		t.Errorf("Expected empty Path, got %q", crumb.Path)
	}
}

// Test struct field types and visibility
func TestThemeData_FieldTypes(t *testing.T) {
	t.Run("struct_field_types", func(t *testing.T) {
		dataType := reflect.TypeOf(ThemeData{})

		// Check field count
		if dataType.NumField() != 7 {
			t.Errorf("Expected 7 fields in ThemeData, got %d", dataType.NumField())
		}

		// Check specific field types
		pathField, found := dataType.FieldByName("Path")
		if !found {
			t.Error("Path field not found")
		} else if pathField.Type.Kind() != reflect.String {
			t.Errorf("Expected Path field to be string, got %v", pathField.Type.Kind())
		}

		parentField, found := dataType.FieldByName("Parent")
		if !found {
			t.Error("Parent field not found")
		} else if parentField.Type.Kind() != reflect.Bool {
			t.Errorf("Expected Parent field to be bool, got %v", parentField.Type.Kind())
		}

		fileCountField, found := dataType.FieldByName("FileCount")
		if !found {
			t.Error("FileCount field not found")
		} else if fileCountField.Type.Kind() != reflect.Int {
			t.Errorf("Expected FileCount field to be int, got %v", fileCountField.Type.Kind())
		}
	})
}

func TestFileItem_FieldTypes(t *testing.T) {
	t.Run("struct_field_types", func(t *testing.T) {
		fileType := reflect.TypeOf(FileItem{})

		// Check field count
		if fileType.NumField() != 5 {
			t.Errorf("Expected 5 fields in FileItem, got %d", fileType.NumField())
		}

		// Check Size field type
		sizeField, found := fileType.FieldByName("Size")
		if !found {
			t.Error("Size field not found")
		} else if sizeField.Type.Kind() != reflect.Int64 {
			t.Errorf("Expected Size field to be int64, got %v", sizeField.Type.Kind())
		}

		// Check IsDir field type
		isDirField, found := fileType.FieldByName("IsDir")
		if !found {
			t.Error("IsDir field not found")
		} else if isDirField.Type.Kind() != reflect.Bool {
			t.Errorf("Expected IsDir field to be bool, got %v", isDirField.Type.Kind())
		}
	})
}

// Benchmark struct creation and access
func BenchmarkThemeData_Creation(b *testing.B) {
	for range b.N {
		_ = ThemeData{
			Path:        "/test",
			Parent:      true,
			FileCount:   10,
			Files:       make([]FileItem, 0),
			Breadcrumbs: make([]BreadcrumbItem, 0),
			CSS:         "/* css */",
			JS:          "// js",
		}
	}
}

func BenchmarkFileItem_Creation(b *testing.B) {
	for range b.N {
		_ = FileItem{
			Name:          "test.txt",
			IsDir:         false,
			Size:          1024,
			FormattedSize: "1.0 KB",
			FormattedTime: "2023-01-01 12:00:00",
		}
	}
}

func BenchmarkBreadcrumbItem_Creation(b *testing.B) {
	for range b.N {
		_ = BreadcrumbItem{
			Name: "Test",
			Path: "/test",
		}
	}
}
