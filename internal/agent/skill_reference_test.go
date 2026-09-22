package agent_test

import (
	"context"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"fusionaly/internal/agent"
	"fusionaly/internal/testsupport"
)

// The plugin skill teaches agents these queries. They must keep passing the
// validator and running on the real schema, or agents learn broken SQL.
func TestSkillReferenceQueriesRun(t *testing.T) {
	dbManager, _ := testsupport.SetupTestDBManager(t)
	db := dbManager.GetConnection()
	doc, err := os.ReadFile("../../plugin/skills/fusionaly/reference.md")
	require.NoError(t, err)
	blocks := regexp.MustCompile("(?s)```sql\n(.*?)```").FindAllStringSubmatch(string(doc), -1)
	require.NotEmpty(t, blocks)

	for _, block := range blocks {
		_, err := agent.Query(context.Background(), db, block[1], 5*time.Second)

		require.NoError(t, err, block[1])
	}
}
