package apiserver

import (
	"context"
	"math/rand"
	"os"
	"testing"
	"time"

	wafiev1 "github.com/Dimss/wafie/api/gen/wafie/v1"
	"github.com/Dimss/wafie/apisrv/internal/models"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func setupTest(t *testing.T) (testcontainers.Container, *gorm.DB, *zap.Logger) {

	ctx := context.Background()
	var pgPort nat.Port = "5432/tcp"
	req := testcontainers.ContainerRequest{
		Image: "postgres:15-alpine",
		HostConfigModifier: func(hostConfig *container.HostConfig) {
			hostConfig.PortBindings = nat.PortMap{
				"5432/tcp": []nat.PortBinding{
					{HostIP: "0.0.0.0", HostPort: "5431"},
				},
			}
		},
		ExposedPorts: []string{string(pgPort)},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForListeningPort(pgPort),
	}
	postgresContainer, err := testcontainers.
		GenericContainer(
			ctx,
			testcontainers.GenericContainerRequest{
				ContainerRequest: req,
				Started:          true,
			},
		)
	if err != nil {
		t.Fatalf("failed to start PostgreSQL container: %v", err)
	}
	// Get the container's host and port
	host, _ := postgresContainer.Host(ctx)
	logger, _ := zap.NewDevelopment()
	dbCfg := models.NewDbCfg(host, 5431, "test", "test", "testdb", logger)
	db, err := models.NewDb(dbCfg)
	if err != nil {
		t.Fatalf("failed to connect to PostgreSQL: %v", err)
	}
	return postgresContainer, db, logger
}

func createApp(db *gorm.DB, logger *zap.Logger) *models.Application {
	appModelSvc := models.NewApplicationRepository(db, logger)
	app, _ := appModelSvc.CreateApplication(&wafiev1.CreateApplicationRequest{Name: "testapp"})
	ingressModelSvc := models.NewIngressRepository(db, logger)
	_ = ingressModelSvc.NewIngressFromRequest(&wafiev1.CreateIngressRequest{Ingress: &wafiev1.Ingress{
		Name:          "testapp",
		Host:          "testapp-host",
		Port:          80,
		Path:          "/",
		UpstreamHost:  "testapp-host",
		UpstreamPort:  8080,
		ApplicationId: int32(app.ID),
	}})
	return app
}

func randomString() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, 10)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func setupTest1() {

	ctx := context.Background()
	var pgPort nat.Port = "5432/tcp"
	req := testcontainers.ContainerRequest{
		Image: "postgres:15-alpine",
		HostConfigModifier: func(hostConfig *container.HostConfig) {
			hostConfig.PortBindings = nat.PortMap{
				"5432/tcp": []nat.PortBinding{
					{HostIP: "0.0.0.0", HostPort: "5431"},
				},
			}
		},
		ExposedPorts: []string{string(pgPort)},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForListeningPort(pgPort),
	}
	postgresContainer, err := testcontainers.
		GenericContainer(
			ctx,
			testcontainers.GenericContainerRequest{
				ContainerRequest: req,
				Started:          true,
			},
		)
	if err != nil {
		panic(err)
	}
	// Get the container's host and port
	host, _ := postgresContainer.Host(ctx)
	logger, _ := zap.NewDevelopment()
	dbCfg := models.NewDbCfg(host, 5431, "test", "test", "testdb", logger)
	_, err = models.NewDb(dbCfg)
	if err != nil {
		panic(err)
	}
}

func TestMain(m *testing.M) {
	setupTest1()
	code := m.Run()
	os.Exit(code)
}
