package main

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/tskinn/ecs-task-tracker/src/utils"
	"github.com/labstack/echo"
)

// SNSMiddleware checks for an sns subscription header and subscribes and short circuits the request
func SNSMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		messageType := c.Request().Header.Get("x-amz-sns-message-type")
		if messageType == "SubscriptionConfirmation" {
			notification, err := utils.DecodeNotification(c.Request().Body)
			if err != nil {
				return c.String(500, "error failed to decode notification: " + err.Error())
			}
			if _, err := http.Get(notification.SubscribeURL); err != nil {
				return c.String(500, "error failed to visit subscribeURL: " + notification.SubscribeURL)
			}
			return c.String(200, "subscribed to sns")
		}
		return next(c)
	}
}

func main() {

	sess := session.Must(session.NewSession(&aws.Config{Region: aws.String(os.Getenv("REGION"))}))
	// TODO get max tries from env var
	// or change it to a exponential backoff limit
	// Must call utils.Init in order for anything in utils to work properly!
	utils.Init(os.Getenv("TRAEFIK_TABLE"),
		os.Getenv("CLUSTER"),
		10,
		dynamodb.New(sess),
		ec2.New(sess),
		ecs.New(sess),
		os.Getenv("DEBUG"),
	)

	e := echo.New()
	e.Use(SNSMiddleware)
	e.GET("/diff", diffAll)
	e.GET("/diff/:service", diff)
	e.POST("/event", ecsEvent)
	e.GET("/sync", syncAll)
	e.GET("/sync/:service", sync)
	e.GET("/syncslow/:milliseconds", syncSlow)
	e.GET("/syncslow", syncSlow)
	e.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "Healthy")
	})
	// TODO add a build endpoint
	e.Logger.Fatal(e.Start(os.Getenv("PORT")))
}

// ecs Event handles SNS messages in the form of http POST requests
func ecsEvent(c echo.Context) error {

	snsType := c.Request().Header.Get("x-amz-sns-message-type")
	messageID := c.Request().Header.Get("x-amz-sns-message-id")
	if snsType == "Notification" {
		err := utils.HandleSNS(messageID, c.Request().Body)
		if err != nil {
			return c.String(500, err.Error())
		}
	}
	return c.String(200, "ecs event processes successfully")
}

// used for testing the slow sync functionality
func syncSlow(c echo.Context) error {
	milliseconds, err := strconv.Atoi(c.Param("milliseconds"))
	if c.Param("milliseconds") == "" {
		milliseconds = 1000
	} else if err != nil {
		return c.String(500, ":<()")
	}
	go utils.HandleSyncSlow(milliseconds)
	return c.String(200, "syncing a service every "+strconv.Itoa(milliseconds)+" milliseconds")
}

// used for testing the sync functionality
func sync(c echo.Context) error {
	serviceName := c.Param("service")
	err := utils.HandleSync(serviceName)
	if err != nil {
		return c.String(500, err.Error())
	}
	return c.String(200, serviceName+" synced")
}

// used for testing the sync all functionality
func syncAll(c echo.Context) error {
	err := utils.HandleSyncAll()
	if err != nil {
		return c.String(500, "error syncing services")
	}
	return c.String(200, "all services synced")
}

func diff(c echo.Context) error {
	serviceName := c.Param("service")
	insync, err := utils.HandleDiff(serviceName)
	if err != nil {
		return c.String(500, err.Error())
	}
	if insync {
		return c.String(200, " is in sync")
	}
	return c.String(200, serviceName+" is out of sync")
}

func diffAll(c echo.Context) error {
	outOfSyncServices, err := utils.HandleDiffAll()
	if err != nil {
		return c.String(500, "error comparing services: "+err.Error())
	}
	if len(outOfSyncServices) == 0 {
		return c.String(200, "all services in sync")
	}
	outString := strings.Join(outOfSyncServices, "\n")
	return c.String(200, "services out of sync:\n"+outString)
}
