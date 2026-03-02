- [ ] Remove all SQL migration code and only generate the go code this is a different project now
Leave the migrations of .schema_snapshot.yaml in the "init".
Remove subcommands init_sql, sql_migrations, migrate_to_go, goose

- [ ] dump_sql should just generate the SQL for the migrations missing so we can see what will happen.
