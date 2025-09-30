#!/bin/bash

# List of providers to fix
PROVIDERS=(
    "auroradsql"
    "tidb"
    "vertica"
    "turso"
    "clickhouse"
)

for provider in "${PROVIDERS[@]}"; do
    provider_file="/workspaces/ocom/go/makemigrations/internal/providers/${provider}/provider.go"

    if [ ! -f "$provider_file" ]; then
        echo "Provider file not found: $provider_file"
        continue
    fi

    echo "Fixing provider: $provider"

    # Check if the file has the bug (DEFAULT + field.Default)
    if grep -q "DEFAULT.*field\.Default" "$provider_file"; then
        echo "  - Found bug in $provider, applying fix..."

        # 1. Add providers import if not already present
        if ! grep -q "github.com/ocomsoft/makemigrations/internal/providers" "$provider_file"; then
            sed -i '/github.com\/ocomsoft\/makemigrations\/internal\/types/i\\t"github.com/ocomsoft/makemigrations/internal/providers"' "$provider_file"
        fi

        # 2. Update convertField function signature to accept schema
        sed -i 's/func (p \*Provider) convertField(field \*types\.Field)/func (p *Provider) convertField(schema *types.Schema, field *types.Field)/' "$provider_file"

        # 3. Update convertField calls to pass schema
        sed -i 's/p\.convertField(&field)/p.convertField(schema, \&field)/' "$provider_file"

        # 4. Replace the default value handling
        sed -i 's/def\.WriteString(" DEFAULT " + field\.Default)/defaultValue := providers.ConvertDefaultValue(schema, "'$provider'", field.Default)\n\t\tdef.WriteString(" DEFAULT " + defaultValue)/' "$provider_file"

        echo "  - Applied fix to $provider"
    else
        echo "  - No bug found in $provider or already fixed"
    fi
done

echo "Provider fixes completed!"