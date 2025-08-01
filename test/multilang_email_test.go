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
