# test_local.py - Para probar localmente
import json
import sys
import os

# Agregar el directorio actual al path
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

def test_message(message_text="Hola", phone="+51999999999"):
    """Simula un mensaje de WhatsApp"""
    
    # Simular evento de Twilio
    event = {
        'body': f'From=whatsapp%3A{phone}&Body={message_text.replace(" ", "%20")}&ProfileName=Test%20User',
        'headers': {'Content-Type': 'application/x-www-form-urlencoded'}
    }
    
    # Importar y ejecutar
    from lambda_function import lambda_handler
    
    try:
        response = lambda_handler(event, {})
        
        # Parsear respuesta XML
        if response['statusCode'] == 200:
            body = response['body']
            # Extraer solo el texto del mensaje
            start = body.find('<Body>') + 6
            end = body.find('</Body>')
            message = body[start:end] if start > 5 and end > 0 else body
            
            print(f"\n📱 Usuario: {message_text}")
            print(f"🤖 Bot: {message}")
            print("-" * 50)
        else:
            print(f"❌ Error: {response}")
            
    except Exception as e:
        print(f"❌ Error al ejecutar: {str(e)}")
        import traceback
        traceback.print_exc()

if __name__ == "__main__":
    # Probar diferentes mensajes
    print("=" * 50)
    print("🧪 PROBANDO CHATBOT LOCALMENTE")
    print("=" * 50)
    
    # Tests básicos
    test_message("Hola")
    test_message("quiero información sobre los cursos")
    test_message("cual es el precio")
    test_message("1")  # Opción desde cero
    test_message("como pago")