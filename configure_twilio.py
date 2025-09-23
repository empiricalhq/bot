# configure_twilio.py
from twilio.rest import Client
import os

# Credenciales de Twilio (desde tu dashboard)
account_sid = 'ACc7abbc8281064d40ef610ea820e54bf3'  # Tu Account SID
auth_token = 'ee8f96965b6e889f8e36a09d34895376'  # Tu Auth Token
api_gateway_url = 'https://07n82oa1a9.execute-api.us-east-1.amazonaws.com/'

client = Client(account_sid, auth_token)

# Para sandbox (desarrollo)
sandbox = client.messaging.services.create(
    friendly_name='Chatbot Magally Dev'
)

# Configurar webhook
webhook_config = client.messaging.services(sandbox.sid).update(
    inbound_request_url=api_gateway_url,
    inbound_method='POST',
    fallback_url=api_gateway_url,
    fallback_method='POST'
)

print(f"Webhook configurado: {api_gateway_url}")
print(f"Número de WhatsApp sandbox: +14155238886")
print("Envía 'join [tu-palabra]' para empezar a probar")