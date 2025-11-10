package test

import (
	"testing"

	"github.com/emprius/emprius-app-backend/notifications/mailtemplates"
	"github.com/stretchr/testify/assert"
)

func TestMultiLanguageEmailTemplates(t *testing.T) {
	// Load templates
	err := mailtemplates.Load()
	assert.NoError(t, err)

	// Test data for template execution
	testData := struct {
		AppName      string
		AppUrl       string
		LogoURL      string
		ToolName     string
		FromDate     string
		ToDate       string
		ButtonUrl    string
		UserName     string
		UserUrl      string
		UserRating   string
		WayOfContact string
		Comment      string
	}{
		AppName:      "ComunPop",
		AppUrl:       "https://app.emprius.cat",
		LogoURL:      "https://app.emprius.cat/assets/logos/banner.png",
		ToolName:     "Test Tool",
		FromDate:     "01 Jan 2024",
		ToDate:       "02 Jan 2024",
		ButtonUrl:    "https://app.emprius.cat/bookings/123",
		UserName:     "Test User",
		UserUrl:      "https://app.emprius.cat/users/123",
		UserRating:   "★★★★★",
		WayOfContact: "Email",
		Comment:      "Test comment",
	}

	t.Run("WelcomeMailNotification", func(t *testing.T) {
		// Test English (default)
		notification, err := mailtemplates.WelcomeMailNotification.ExecTemplate(testData, "en")
		assert.NoError(t, err)
		assert.Contains(t, notification.Subject, "Welcome to ComunPop")
		assert.Contains(t, notification.Body, "Thank You for Registering!")

		// Test Spanish
		notification, err = mailtemplates.WelcomeMailNotification.ExecTemplate(testData, "es")
		assert.NoError(t, err)
		assert.Contains(t, notification.Subject, "Bienvenido a ComunPop")
		assert.Contains(t, notification.Body, "¡Gracias por Registrarte!")

		// Test Catalan
		notification, err = mailtemplates.WelcomeMailNotification.ExecTemplate(testData, "ca")
		assert.NoError(t, err)
		assert.Contains(t, notification.Subject, "Benvingut a ComunPop")
		assert.Contains(t, notification.Body, "Gràcies per Registrar-te!")

		// Test fallback to English for unsupported language
		notification, err = mailtemplates.WelcomeMailNotification.ExecTemplate(testData, "fr")
		assert.NoError(t, err)
		assert.Contains(t, notification.Subject, "Welcome to ComunPop")
		assert.Contains(t, notification.Body, "Thank You for Registering!")
	})

	t.Run("BookingAcceptedMailNotification", func(t *testing.T) {
		// Test English (default)
		notification, err := mailtemplates.BookingAcceptedMailNotification.ExecTemplate(testData, "en")
		assert.NoError(t, err)
		assert.Contains(t, notification.Subject, "Your tool request has been accepted")
		assert.Contains(t, notification.Body, "Your Request Has Been Accepted!")

		// Test Spanish
		notification, err = mailtemplates.BookingAcceptedMailNotification.ExecTemplate(testData, "es")
		assert.NoError(t, err)
		assert.Contains(t, notification.Subject, "Tu solicitud de herramienta ha sido aceptada")
		assert.Contains(t, notification.Body, "¡Tu Solicitud Ha Sido Aceptada!")

		// Test Catalan
		notification, err = mailtemplates.BookingAcceptedMailNotification.ExecTemplate(testData, "ca")
		assert.NoError(t, err)
		assert.Contains(t, notification.Subject, "La teva sol·licitud d'eina ha estat acceptada")
		assert.Contains(t, notification.Body, "La Teva Sol·licitud Ha Estat Acceptada!")
	})

	t.Run("NewIncomingRequestMailNotification", func(t *testing.T) {
		// Test English (default)
		notification, err := mailtemplates.NewIncomingRequestMailNotification.ExecTemplate(testData, "en")
		assert.NoError(t, err)
		assert.Contains(t, notification.Subject, "New Tool request received")
		assert.Contains(t, notification.Body, "Tool Borrow Request")

		// Test Spanish
		notification, err = mailtemplates.NewIncomingRequestMailNotification.ExecTemplate(testData, "es")
		assert.NoError(t, err)
		assert.Contains(t, notification.Subject, "Nueva solicitud de herramienta recibida")
		assert.Contains(t, notification.Body, "Solicitud de Préstamo de Herramienta")

		// Test Catalan
		notification, err = mailtemplates.NewIncomingRequestMailNotification.ExecTemplate(testData, "ca")
		assert.NoError(t, err)
		assert.Contains(t, notification.Subject, "Nova sol·licitud d'eina rebuda")
		assert.Contains(t, notification.Body, "Sol·licitud de Préstec d'Eina")
	})

	t.Run("NomadicToolHolderIsChangedMailNotification", func(t *testing.T) {
		// Test English (default)
		notification, err := mailtemplates.NomadicToolHolderIsChangedMailNotification.ExecTemplate(testData, "en")
		assert.NoError(t, err)
		assert.Contains(t, notification.Subject, "Nomadic tool holder has been changed")
		assert.Contains(t, notification.Body, "Nomadic tool holder has been changed")

		// Test Spanish
		notification, err = mailtemplates.NomadicToolHolderIsChangedMailNotification.ExecTemplate(testData, "es")
		assert.NoError(t, err)
		assert.Contains(t, notification.Subject, "El portador de la herramienta nómada ha cambiado")
		assert.Contains(t, notification.Body, "El portador de la herramienta nómada ha cambiado")

		// Test Catalan
		notification, err = mailtemplates.NomadicToolHolderIsChangedMailNotification.ExecTemplate(testData, "ca")
		assert.NoError(t, err)
		assert.Contains(t, notification.Subject, "El portador de l'eina nòmada ha canviat")
		assert.Contains(t, notification.Body, "El portador de l'eina nòmada ha canviat")
	})
}

func TestAddCommonTemplateFields(t *testing.T) {
	// Load templates
	err := mailtemplates.Load()
	assert.NoError(t, err)

	t.Run("StructFieldExtraction", func(t *testing.T) {
		// Test that all exported struct fields are correctly extracted
		testData := struct {
			AppName  string
			ToolName string
			UserName string
			AppUrl   string
		}{
			AppName:  "TestApp",
			ToolName: "TestTool",
			UserName: "TestUser",
			AppUrl:   "https://test.example.com",
		}

		notification, err := mailtemplates.WelcomeMailNotification.ExecTemplate(testData, "en")
		assert.NoError(t, err)
		assert.NotNil(t, notification)

		// Verify all struct fields are available in the template
		assert.Contains(t, notification.Body, "TestApp")
		assert.Contains(t, notification.Body, "test.example.com")
	})

	t.Run("NotificationsUrlAddedWhenNotPresent", func(t *testing.T) {
		// Test that NotificationsUrl is added when the struct doesn't have it
		// Use BookingAcceptedMailNotification because it uses {{.NotificationsUrl}} in the template
		testData := struct {
			AppName   string
			AppUrl    string
			LogoURL   string
			ToolName  string
			FromDate  string
			ToDate    string
			ButtonUrl string
			UserName  string
			UserUrl   string
		}{
			AppName:   "TestApp",
			AppUrl:    "https://test.example.com",
			LogoURL:   "https://test.example.com/logo.png",
			ToolName:  "TestTool",
			FromDate:  "01 Jan 2024",
			ToDate:    "02 Jan 2024",
			ButtonUrl: "https://test.example.com/bookings/123",
			UserName:  "TestUser",
			UserUrl:   "https://test.example.com/users/123",
		}

		notification, err := mailtemplates.BookingAcceptedMailNotification.ExecTemplate(testData, "en")
		assert.NoError(t, err)
		assert.NotNil(t, notification)

		// The template should have access to the default NotificationsUrl
		// BookingAcceptedMailNotification template uses NotificationsUrl in its HTML,
		// so it should contain the default value
		assert.Contains(t, notification.Body, "app.emprius.cat/profile#notifications")
	})

	t.Run("NotificationsUrlNotOverriddenWhenPresent", func(t *testing.T) {
		// Test that NotificationsUrl is NOT overridden if already present in struct
		customNotificationsUrl := "https://custom.example.com/my-notifications"
		testData := struct {
			AppName          string
			AppUrl           string
			LogoURL          string
			ToolName         string
			FromDate         string
			ToDate           string
			ButtonUrl        string
			UserName         string
			UserUrl          string
			NotificationsUrl string
		}{
			AppName:          "TestApp",
			AppUrl:           "https://test.example.com",
			LogoURL:          "https://test.example.com/logo.png",
			ToolName:         "TestTool",
			FromDate:         "01 Jan 2024",
			ToDate:           "02 Jan 2024",
			ButtonUrl:        "https://test.example.com/bookings/123",
			UserName:         "TestUser",
			UserUrl:          "https://test.example.com/users/123",
			NotificationsUrl: customNotificationsUrl,
		}

		notification, err := mailtemplates.BookingAcceptedMailNotification.ExecTemplate(testData, "en")
		assert.NoError(t, err)
		assert.NotNil(t, notification)

		// Verify the custom NotificationsUrl is preserved and not overridden
		assert.Contains(t, notification.Body, customNotificationsUrl)
		assert.Contains(t, notification.Body, "custom.example.com/my-notifications")

		// Verify the default URL is NOT present
		assert.NotContains(t, notification.Body, "app.emprius.cat/profile#notifications")
	})

	t.Run("PointerHandling", func(t *testing.T) {
		// Test that pointer to struct is correctly handled
		testData := struct {
			AppName   string
			AppUrl    string
			LogoURL   string
			ToolName  string
			FromDate  string
			ToDate    string
			ButtonUrl string
			UserName  string
			UserUrl   string
		}{
			AppName:   "PointerTestApp",
			AppUrl:    "https://pointer.test.example.com",
			LogoURL:   "https://pointer.test.example.com/logo.png",
			ToolName:  "PointerTestTool",
			FromDate:  "01 Jan 2024",
			ToDate:    "02 Jan 2024",
			ButtonUrl: "https://pointer.test.example.com/bookings/123",
			UserName:  "PointerTestUser",
			UserUrl:   "https://pointer.test.example.com/users/123",
		}

		// Pass as pointer instead of value
		notification, err := mailtemplates.BookingAcceptedMailNotification.ExecTemplate(&testData, "en")
		assert.NoError(t, err)
		assert.NotNil(t, notification)

		// Verify that fields from pointer are correctly extracted
		assert.Contains(t, notification.Body, "PointerTestApp")
		assert.Contains(t, notification.Body, "pointer.test.example.com")

		// Also verify NotificationsUrl was added
		assert.Contains(t, notification.Body, "app.emprius.cat/profile#notifications")
	})
}
