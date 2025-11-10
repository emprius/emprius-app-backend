package mailtemplates

import (
	"bytes"
	"fmt"
	htmltemplate "html/template"
	"reflect"
	"strings"
	texttemplate "text/template"

	root "github.com/emprius/emprius-app-backend"
	"github.com/emprius/emprius-app-backend/notifications"
)

// availableTemplates is a map that stores the filename and the absolute path
// of the email templates. The filename is the key and the path is the value.
var availableTemplates map[TemplateFile]string

// availableTemplatesLang is a map that stores templates by language
// Structure: map[TemplateFile]map[language]path
var availableTemplatesLang map[TemplateFile]map[string]string

// TemplateFile represents an email template key. Every email template should
// have a key that identifies it, which is the filename without the extension.
type TemplateFile string

// MailTemplate struct represents an email template. It includes the file key
// and the notification placeholder to be sent. The file key is the filename
// of the template without the extension. The notification placeholder includes
// the plain body template to be used as a fallback for email clients that do
// not support HTML, and the mail subject.
type MailTemplate struct {
	File        TemplateFile
	Placeholder notifications.Notification
	WebAppURI   string
	// Multi-language support for subjects and plain text
	Subjects    map[string]string // language -> subject
	PlainBodies map[string]string // language -> plain body
}

// Available function returns the available email templates. It returns a map
// with the filename and the absolute path of the email templates. The filename
// is the key and the path is the value.
func Available() map[TemplateFile]string {
	return availableTemplates
}

// Load function reads the email templates from embedded assets. It reads the
// html files from the "assets" directory and stores the filename and the file
// path in the availableTemplates map. It returns an error if the directory
// could not be read or if the files could not be read.
func Load() error {
	// reset the maps to store the filename and file paths
	availableTemplates = make(map[TemplateFile]string)
	availableTemplatesLang = make(map[TemplateFile]map[string]string)

	// read files from embedded assets
	entries, err := root.Assets.ReadDir("assets")
	if err != nil {
		return err
	}

	// walk through the directory and read each file
	for _, entry := range entries {
		// only process regular files and files with a ".html" extension
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".html") {
			// extract template name and language from filename
			nameWithoutExt := strings.TrimSuffix(entry.Name(), ".html")

			// check if filename contains language suffix (e.g., "welcome_en", "welcome_es")
			parts := strings.Split(nameWithoutExt, "_")
			if len(parts) >= 2 {
				// extract language code (last part) and template name (everything before)
				langCode := parts[len(parts)-1]
				templateName := strings.Join(parts[:len(parts)-1], "_")
				templateFile := TemplateFile(templateName)

				// initialize language map for this template if it doesn't exist
				if availableTemplatesLang[templateFile] == nil {
					availableTemplatesLang[templateFile] = make(map[string]string)
				}

				// store the language-specific template path
				availableTemplatesLang[templateFile][langCode] = "assets/" + entry.Name()

				// for backward compatibility, store English templates in the old map
				if langCode == "en" {
					availableTemplates[templateFile] = "assets/" + entry.Name()
				}
			} else {
				// fallback for templates without language suffix (backward compatibility)
				availableTemplates[TemplateFile(nameWithoutExt)] = "assets/" + entry.Name()
			}
		}
	}
	return nil
}

// addCommonTemplateFields adds common fields to the template data that should be
// available to all templates (e.g., NotificationsUrl). It converts the data
// to a map and adds the common fields. If data is already a map, it adds to it.
// If data is a struct, it uses reflection to copy all exported fields.
func addCommonTemplateFields(data any) map[string]any {
	result := make(map[string]any)

	// convert data to map if it's a map[string]any
	if dataMap, ok := data.(map[string]any); ok {
		for k, v := range dataMap {
			result[k] = v
		}
	} else {
		// use reflection to copy struct fields to map
		val := reflect.ValueOf(data)
		typ := reflect.TypeOf(data)

		// handle pointers
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
			typ = typ.Elem()
		}

		// if it's a struct, copy all exported fields
		if val.Kind() == reflect.Struct {
			for i := 0; i < val.NumField(); i++ {
				field := typ.Field(i)
				// only copy exported fields (fields that start with uppercase)
				if field.PkgPath == "" {
					result[field.Name] = val.Field(i).Interface()
				}
			}
		}
	}

	// add common fields available to all templates
	result["NotificationsUrl"] = NotificationsUrl

	return result
}

// ExecTemplate method checks if the template file exists in the available
// mail templates and if it does, it executes the template with the data
// provided. If it doesn't exist, it returns an error. If the plain body
// placeholder is not empty, it executes the plain text template with the
// data provided. It returns the notification with the body and plain body
// filled with the data provided.
// The optional lang parameter specifies the language code (e.g., "en", "es", "ca").
// If not provided or not supported, defaults to English ("en").
func (mt MailTemplate) ExecTemplate(data any, lang string) (*notifications.Notification, error) {
	// try to find the language-specific template first
	var path string
	var ok bool

	if langTemplates, exists := availableTemplatesLang[mt.File]; exists {
		if langPath, langExists := langTemplates[lang]; langExists {
			path = langPath
			ok = true
		} else if langPath, langExists := langTemplates["en"]; langExists {
			// fallback to English if requested language not available
			path = langPath
			ok = true
		}
	}

	// fallback to old template system for backward compatibility
	if !ok {
		path, ok = availableTemplates[mt.File]
	}

	if !ok {
		return nil, fmt.Errorf("template not found")
	}

	// enrich template data with common fields
	enrichedData := addCommonTemplateFields(data)

	// create a notification with the plain body placeholder inflated
	n, err := mt.ExecPlain(enrichedData, lang)
	if err != nil {
		return nil, err
	}

	// parse the html template file
	content, err := root.Assets.ReadFile(path)
	if err != nil {
		return nil, err
	}
	tmpl, err := htmltemplate.New(string(mt.File)).Parse(string(content))
	if err != nil {
		return nil, err
	}
	// inflate the template with the data
	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, enrichedData); err != nil {
		return nil, err
	}
	// set the body of the notification
	n.Body = buf.String()
	return n, nil
}

// ExecPlain method executes the plain body placeholder template with the data
// provided using the specified language. If the placeholder plain body is not empty, it executes the plain
// text template with the data provided. If it is empty, just returns an empty
// notification. It resulting notification and an error if the defined template
// could not be executed.
//
// If language-specific templates are available,
// it uses them; otherwise, it falls back to the default placeholder templates.
//
// This method also allows to notifications services that do not support HTML
// emails to use a mail template.
func (mt MailTemplate) ExecPlain(data any, languageCode string) (*notifications.Notification, error) {
	n := &notifications.Notification{}
	// Try to use language-specific plain body first
	var plainBodyTemplate string
	if mt.PlainBodies != nil && mt.PlainBodies[languageCode] != "" {
		plainBodyTemplate = mt.PlainBodies[languageCode]
	} else if mt.PlainBodies != nil && mt.PlainBodies["en"] != "" {
		// fallback to English
		plainBodyTemplate = mt.PlainBodies["en"]
	} else if mt.Placeholder.PlainBody != "" {
		// fallback to original placeholder
		plainBodyTemplate = mt.Placeholder.PlainBody
	}

	if plainBodyTemplate != "" {
		// parse the plain body template
		tmpl, err := texttemplate.New("plain").Parse(plainBodyTemplate)
		if err != nil {
			return nil, err
		}
		// inflate the template with the data
		buf := new(bytes.Buffer)
		if err := tmpl.Execute(buf, data); err != nil {
			return nil, err
		}
		// return the notification with the plain body filled with the data
		n.PlainBody = buf.String()
	}

	// Try to use language-specific subject first
	var subjectTemplate string
	if mt.Subjects != nil && mt.Subjects[languageCode] != "" {
		subjectTemplate = mt.Subjects[languageCode]
	} else if mt.Subjects != nil && mt.Subjects["en"] != "" {
		// fallback to English
		subjectTemplate = mt.Subjects["en"]
	} else if mt.Placeholder.Subject != "" {
		// fallback to original placeholder
		subjectTemplate = mt.Placeholder.Subject
	}

	if subjectTemplate != "" {
		// parse the subject template
		tmpl, err := texttemplate.New("subject").Parse(subjectTemplate)
		if err != nil {
			return nil, err
		}
		// inflate the template with the data
		buf := new(bytes.Buffer)
		if err := tmpl.Execute(buf, data); err != nil {
			return nil, err
		}
		// return the notification with the subject filled with the data
		n.Subject = buf.String()
	}

	return n, nil
}
