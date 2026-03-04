- [x] Remove all SQL migration code and only generate the go code this is a different project now
Leave the migrations of .schema_snapshot.yaml in the "init".
Remove subcommands init_sql, sql_migrations, migrate_to_go, goose

- [x] dump_sql should just generate the SQL for the migrations missing so we can see what will happen.
- [ ] Altering a field type can fail. Can we have it have a optional "method" such as "alter_column" or "new_column" which will rename the existing column then create new one and use the old value as a default with type casting and then drop old column and set default properly

- [ ] db2schema and db-diff use provider
