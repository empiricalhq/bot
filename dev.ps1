$region = "us-east-2"

function LogCmd {
    param (
        [string]$command,
        [string[]]$arguments
    )
    Write-Output "Running: $command $arguments"
    Start-Process -FilePath $command -ArgumentList $arguments -NoNewWindow `
        -RedirectStandardOutput "dev.log" -RedirectStandardError "dev.log" -Wait
}

# tabla 1: user state
Write-Output "Setting up DynamoDB tables..."
Write-Output "Table: UserState"
LogCmd "aws" @(
    "dynamodb", "create-table",
    "--table-name", "UserState",
    "--attribute-definitions", "AttributeName=UserID,AttributeType=S",
    "--key-schema", "AttributeName=UserID,KeyType=HASH",
    "--billing-mode", "PAY_PER_REQUEST",
    "--region", $region
)

# tabla 2: historial de conversaciones
Write-Output "Table: ConversationHistory"
LogCmd "aws" @(
    "dynamodb", "create-table",
    "--table-name", "ConversationHistory",
    "--attribute-definitions", "AttributeName=UserID,AttributeType=S", "AttributeName=Timestamp,AttributeType=N",
    "--key-schema", "AttributeName=UserID,KeyType=HASH", "AttributeName=Timestamp,KeyType=RANGE",
    "--billing-mode", "PAY_PER_REQUEST",
    "--region", $region
)

# tabla 3: sesiones
Write-Output "Table: Session"
LogCmd "aws" @(
    "dynamodb", "create-table",
    "--table-name", "Session",
    "--attribute-definitions", "AttributeName=SessionID,AttributeType=S",
    "--key-schema", "AttributeName=SessionID,KeyType=HASH",
    "--billing-mode", "PAY_PER_REQUEST",
    "--region", $region
)
