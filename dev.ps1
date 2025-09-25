# definiciones
$region = "us-east-2"

# creacion de tablas
# tabla 1: user state
Write-Host "Creating DynamoDB table (UserState)..."
aws dynamodb create-table `
  --table-name UserState `
  --attribute-definitions AttributeName=UserID,AttributeType=S `
  --key-schema AttributeName=UserID,KeyType=HASH `
  --billing-mode PAY_PER_REQUEST `
  --region $region

# tabla 2: historial de conversaciones
Write-Host "Creating DynamoDB table (ConversationHistory)..."
aws dynamodb create-table `
  --table-name ConversationHistory `
  --attribute-definitions AttributeName=UserID,AttributeType=S AttributeName=Timestamp,AttributeType=N `
  --key-schema AttributeName=UserID,KeyType=HASH AttributeName=Timestamp,KeyType=RANGE `
  --billing-mode PAY_PER_REQUEST `
  --region $region

# tabla 3: sesiones
Write-Host "Creating DynamoDB table (Session)..."
aws dynamodb create-table `
  --table-name Session `
  --attribute-definitions AttributeName=SessionID,AttributeType=S `
  --key-schema AttributeName=SessionID,KeyType=HASH `
  --billing-mode PAY_PER_REQUEST `
  --region $region
