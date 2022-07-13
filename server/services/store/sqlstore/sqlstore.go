package sqlstore

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mattermost/mattermost-server/v6/plugin"

	sq "github.com/Masterminds/squirrel"

	"github.com/mattermost/focalboard/server/model"
	"github.com/mattermost/focalboard/server/services/store"
	"github.com/mattermost/mattermost-plugin-api/cluster"

	mmModel "github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/shared/mlog"
)

// SQLStore is a SQL database.
type SQLStore struct {
	db               *sql.DB
	dbType           string
	tablePrefix      string
	connectionString string
	isPlugin         bool
	isSingleUser     bool
	logger           *mlog.Logger
	NewMutexFn       MutexFactory
	pluginAPI        *plugin.API
	isBinaryParam    bool
}

// MutexFactory is used by the store in plugin mode to generate
// a cluster mutex.
type MutexFactory func(name string) (*cluster.Mutex, error)

// New creates a new SQL implementation of the store.
func New(params Params) (*SQLStore, error) {
	if err := params.CheckValid(); err != nil {
		return nil, err
	}

	params.Logger.Info("connectDatabase", mlog.String("dbType", params.DBType))
	store := &SQLStore{
		// TODO: add replica DB support too.
		db:               params.DB,
		dbType:           params.DBType,
		tablePrefix:      params.TablePrefix,
		connectionString: params.ConnectionString,
		logger:           params.Logger,
		isPlugin:         params.IsPlugin,
		isSingleUser:     params.IsSingleUser,
		NewMutexFn:       params.NewMutexFn,
		pluginAPI:        params.PluginAPI,
	}

	var err error
	store.isBinaryParam, err = store.computeBinaryParam()
	if err != nil {
		params.Logger.Error(`Cannot compute binary parameter`, mlog.Err(err))
		return nil, err
	}

	err = store.Migrate()
	if err != nil {
		params.Logger.Error(`Table creation / migration failed`, mlog.Err(err))

		return nil, err
	}
	return store, nil
}

// computeBinaryParam returns whether the data source uses binary_parameters
// when using Postgres.
func (s *SQLStore) computeBinaryParam() (bool, error) {
	if s.dbType != model.PostgresDBType {
		return false, nil
	}

	url, err := url.Parse(s.connectionString)
	if err != nil {
		return false, err
	}
	return url.Query().Get("binary_parameters") == "yes", nil
}

// Shutdown close the connection with the store.
func (s *SQLStore) Shutdown() error {
	return s.db.Close()
}

// DBHandle returns the raw sql.DB handle.
// It is used by the mattermostauthlayer to run their own
// raw SQL queries.
func (s *SQLStore) DBHandle() *sql.DB {
	return s.db
}

// DBType returns the DB driver used for the store.
func (s *SQLStore) DBType() string {
	return s.dbType
}

func (s *SQLStore) getQueryBuilder(db sq.BaseRunner) sq.StatementBuilderType {
	builder := sq.StatementBuilder
	if s.dbType == model.PostgresDBType || s.dbType == model.SqliteDBType {
		builder = builder.PlaceholderFormat(sq.Dollar)
	}

	return builder.RunWith(db)
}

func (s *SQLStore) escapeField(fieldName string) string {
	if s.dbType == model.MysqlDBType {
		return "`" + fieldName + "`"
	}
	if s.dbType == model.PostgresDBType || s.dbType == model.SqliteDBType {
		return "\"" + fieldName + "\""
	}
	return fieldName
}

func (s *SQLStore) durationSelector(interval string) string {
	intervalMagnitudeString := strings.Fields(interval)[0]
	intervalMagnitude, err := strconv.Atoi(intervalMagnitudeString)
	if err != nil {
		// handle error
		os.Exit(2)
	}
	if strings.Contains(interval, "day") {
		return time.Now().AddDate(0, 0, -1*intervalMagnitude).Format(time.RFC3339)
	}
	if strings.Contains(interval, "month") {
		return time.Now().AddDate(0, -1*intervalMagnitude, 0).Format(time.RFC3339)
	}
	if strings.Contains(interval, "year") {
		return time.Now().AddDate(-1*intervalMagnitude, 0, 0).Format(time.RFC3339)
	}
	return time.Now().Format(time.RFC3339)
}

func (s *SQLStore) concatenationSelector(field string, delimiter string) string {
	if s.dbType == model.SqliteDBType {
		return fmt.Sprintf("group_concat(%s)", field)
	}
	if s.dbType == model.PostgresDBType {
		return fmt.Sprintf("string_agg(%s, '%s')", field, delimiter)
	}
	if s.dbType == model.MysqlDBType {
		return fmt.Sprintf("GROUP_CONCAT(%s SEPARATOR '%s')", field, delimiter)
	}
	return ""
}

func (s *SQLStore) elementInColumn(parameterCount int, column string) string {
	if s.dbType == model.SqliteDBType || s.dbType == model.MysqlDBType {
		return fmt.Sprintf("instr(%s, %s) > 0", column, s.parameterPlaceholder(parameterCount))
	}
	if s.dbType == model.PostgresDBType {
		return fmt.Sprintf("position(%s in %s) > 0", s.parameterPlaceholder(parameterCount), column)
	}
	return ""
}

func (s *SQLStore) getLicense(db sq.BaseRunner) *mmModel.License {
	return nil
}

func (s *SQLStore) parameterPlaceholder(count int) string {
	if s.dbType == model.PostgresDBType || s.dbType == model.SqliteDBType {
		return fmt.Sprintf("$%v", count)
	}
	if s.dbType == model.MysqlDBType {
		return "?"
	}
	return ""
}

func (s *SQLStore) getCloudLimits(db sq.BaseRunner) (*mmModel.ProductLimits, error) {
	return nil, nil
}

func (s *SQLStore) searchUserChannels(db sq.BaseRunner, teamID, userID, query string) ([]*mmModel.Channel, error) {
	return nil, store.NewNotSupportedError("search user channels not supported on standalone mode")
}

func (s *SQLStore) getChannel(db sq.BaseRunner, teamID, channel string) (*mmModel.Channel, error) {
	return nil, store.NewNotSupportedError("get channel not supported on standalone mode")
}
