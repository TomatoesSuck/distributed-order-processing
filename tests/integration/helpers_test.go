//go:build integration

package integration

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mysql"
	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"
	"github.com/testcontainers/testcontainers-go/wait"

	"net"
)

// projectRoot is set in TestMain; everything depends on this.
var projectRoot string

// pre-built binary paths, populated in TestMain.
var binaries struct {
	order     string
	inventory string
	payment   string
}

// testInfra holds shared MySQL + RabbitMQ for the whole test run.
type testInfra struct {
	mysqlCt   *mysql.MySQLContainer
	rabbitCt  *rabbitmq.RabbitMQContainer
	mysqlHost string
	mysqlPort string
	amqpURL   string
	rootDB    *sql.DB
}

func setupInfra(t *testing.T) *testInfra {
	t.Helper()
	ctx := context.Background()

	mysqlCt, err := mysql.Run(ctx,
		"mysql:8.0",
		mysql.WithDatabase("bootstrap"),
		mysql.WithUsername("root"),
		mysql.WithPassword("rootpw"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("ready for connections").WithStartupTimeout(2*time.Minute),
		),
	)
	require.NoError(t, err, "start mysql container")

	rabbitCt, err := rabbitmq.Run(ctx, "rabbitmq:3-management")
	require.NoError(t, err, "start rabbitmq container")

	host, err := mysqlCt.Host(ctx)
	require.NoError(t, err)
	port, err := mysqlCt.MappedPort(ctx, "3306/tcp")
	require.NoError(t, err)

	rootDSN := fmt.Sprintf("root:rootpw@tcp(%s:%s)/?parseTime=true&loc=UTC", host, port.Port())
	rootDB, err := sql.Open("mysql", rootDSN)
	require.NoError(t, err)
	require.NoError(t, rootDB.Ping())

	for _, s := range []string{"orders_db", "inventory_db", "payments_db"} {
		_, err := rootDB.Exec("CREATE DATABASE IF NOT EXISTS " + s + " CHARACTER SET utf8mb4")
		require.NoError(t, err, "create schema %s", s)
	}

	amqpURL, err := rabbitCt.AmqpURL(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		rootDB.Close()
		_ = mysqlCt.Terminate(context.Background())
		_ = rabbitCt.Terminate(context.Background())
	})

	return &testInfra{
		mysqlCt:   mysqlCt,
		rabbitCt:  rabbitCt,
		mysqlHost: host,
		mysqlPort: port.Port(),
		amqpURL:   amqpURL,
		rootDB:    rootDB,
	}
}

// resetSchemas drops and recreates the three service schemas so each test starts clean.
func (i *testInfra) resetSchemas(t *testing.T) {
	t.Helper()
	for _, s := range []string{"orders_db", "inventory_db", "payments_db"} {
		_, err := i.rootDB.Exec("DROP DATABASE IF EXISTS " + s)
		require.NoError(t, err)
		_, err = i.rootDB.Exec("CREATE DATABASE " + s + " CHARACTER SET utf8mb4")
		require.NoError(t, err)
	}
	// RabbitMQ topology is recreated by each service on startup; queue state
	// (unacked messages) is irrelevant because we kill old service processes between tests.
}

func (i *testInfra) schemaDB(t *testing.T, schema string) *sql.DB {
	t.Helper()
	dsn := fmt.Sprintf("root:rootpw@tcp(%s:%s)/%s?parseTime=true&loc=UTC", i.mysqlHost, i.mysqlPort, schema)
	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	require.NoError(t, db.Ping())
	return db
}

// pickFreePort returns a host port that is free *right now*. Callers should start
// the binding process immediately; there is a race with anyone else grabbing the port.
func pickFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	return port
}

// service is a running subprocess wrapped with its address and a kill hook.
type service struct {
	name   string
	port   int
	cmd    *exec.Cmd
	cancel context.CancelFunc
	once   sync.Once
	stdout io.ReadCloser
}

func (s *service) stop() {
	s.once.Do(func() {
		s.cancel()
		_ = s.cmd.Process.Kill()
		_, _ = s.cmd.Process.Wait()
	})
}

// startService launches a compiled service binary with the given env, then blocks
// until /health responds 200 (or fails the test on timeout).
func startService(t *testing.T, name, bin string, port int, env map[string]string) *service {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, bin)
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	// Pipe stdout to test logs for easier debugging on failure.
	out, err := cmd.StdoutPipe()
	require.NoError(t, err)
	cmd.Stderr = cmd.Stdout

	require.NoError(t, cmd.Start(), "start %s", name)

	svc := &service{name: name, port: port, cmd: cmd, cancel: cancel, stdout: out}

	// Drain output in background so the pipe never fills.
	go func() {
		_, _ = io.Copy(io.Discard, out)
	}()

	t.Cleanup(svc.stop)

	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	require.Eventually(t, func() bool {
		resp, err := http.Get(healthURL)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 30*time.Second, 200*time.Millisecond, "%s health-check at %s never returned 200", name, healthURL)

	return svc
}

// startAllServices brings up order + inventory + payment subprocesses pointing at the
// shared MySQL + RabbitMQ. paymentFailureRate is forwarded to PAYMENT_FAILURE_RATE.
func (i *testInfra) startAllServices(t *testing.T, paymentFailureRate float64) (orderSvc, inventorySvc, paymentSvc *service) {
	t.Helper()

	orderPort := pickFreePort(t)
	invPort := pickFreePort(t)
	payPort := pickFreePort(t)

	commonEnv := func(dbName string) map[string]string {
		return map[string]string{
			"DB_HOST":      i.mysqlHost,
			"DB_PORT":      i.mysqlPort,
			"DB_USER":      "root",
			"DB_PASS":      "rootpw",
			"DB_NAME":      dbName,
			"RABBITMQ_URL": i.amqpURL,
		}
	}

	orderEnv := commonEnv("orders_db")
	orderEnv["ORDER_PORT"] = strconv.Itoa(orderPort)

	invEnv := commonEnv("inventory_db")
	invEnv["INVENTORY_PORT"] = strconv.Itoa(invPort)
	invEnv["INVENTORY_RESERVE_MAX_RETRIES"] = "50"

	payEnv := commonEnv("payments_db")
	payEnv["PAYMENT_PORT"] = strconv.Itoa(payPort)
	payEnv["PAYMENT_FAILURE_RATE"] = strconv.FormatFloat(paymentFailureRate, 'f', -1, 64)

	// Order brings up the saga consumer; inventory/payment bring up their command consumers.
	// Order can come up after the others; startup order doesn't matter because RabbitMQ buffers.
	orderSvc = startService(t, "order", binaries.order, orderPort, orderEnv)
	inventorySvc = startService(t, "inventory", binaries.inventory, invPort, invEnv)
	paymentSvc = startService(t, "payment", binaries.payment, payPort, payEnv)
	return orderSvc, inventorySvc, paymentSvc
}

