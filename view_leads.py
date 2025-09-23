import json
import base64
import boto3
from urllib.parse import parse_qs
from datetime import datetime
from decimal import Decimal

# Configuración
dynamodb = boto3.resource('dynamodb')
table = dynamodb.Table('chatbot-leads')

def get_user_data(phone):
    """Obtiene el historial completo del usuario"""
    try:
        response = table.query(
            KeyConditionExpression='phone_number = :phone',
            ExpressionAttributeValues={':phone': phone},
            ScanIndexForward=False  # Más reciente primero
        )
        if response['Items']:
            # Buscar el nombre en todo el historial
            user_name = None
            for item in response['Items']:
                if item.get('user_name'):
                    user_name = item.get('user_name')
                    break
            
            # Retornar el más reciente con el nombre encontrado
            latest = response['Items'][0]
            if user_name:
                latest['user_name'] = user_name
            return latest
    except Exception as e:
        print(f"Error obteniendo datos: {e}")
    return {}

def save_interaction(phone, message, response_text, stage, user_name=None):
    """Guarda interacción en DynamoDB"""
    try:
        item = {
            'phone_number': phone,
            'timestamp': Decimal(str(datetime.now().timestamp())),
            'user_message': message,
            'bot_response': response_text[:500],
            'stage': stage,
            'date': datetime.now().isoformat()
        }
        if user_name:
            item['user_name'] = user_name
        
        table.put_item(Item=item)
        print(f"✅ Guardado: {phone} - {stage} - Nombre: {user_name}")
        return True
    except Exception as e:
        print(f"❌ Error guardando: {e}")
        return False

def lambda_handler(event, context):
    try:
        # Decodificar mensaje
        body_str = event.get('body', '')
        if event.get('isBase64Encoded', False):
            body_str = base64.b64decode(body_str).decode('utf-8')
        
        body = parse_qs(body_str)
        message = body.get('Body', [''])[0]
        phone = body.get('From', [''])[0].replace('whatsapp:', '').replace('%3A', ':').replace('+', '')
        
        print(f"📱 Mensaje de {phone}: '{message}'")
        
        msg = message.lower().strip()
        
        # Obtener datos previos
        user_data = get_user_data(phone)
        user_name = user_data.get('user_name', '')
        current_stage = user_data.get('stage', 'NUEVO')
        
        print(f"👤 Usuario: {user_name if user_name else 'Sin nombre'}, Stage: {current_stage}")
        
        response_text = ""
        new_stage = current_stage
        
        # ============= CAPTURA DE NOMBRE MEJORADA =============
        if current_stage == 'ESPERANDO_NOMBRE' and not user_name:
            # Asumimos que cualquier texto aquí es el nombre
            user_name = message.strip().title()
            response_text = f"""¡Mucho gusto *{user_name}*! 😊

Tenemos más de 15 años formando alumnas en toda América Latina 🌎

¿Ya sabes joyería tejida o deseas empezar desde cero?

*1* → Desde Cero (principiantes)
*2* → Ya tengo experiencia
*PRECIO* → Ver precios directamente"""
            new_stage = 'NOMBRE_CAPTURADO'
            
            # Guardar con nombre
            save_interaction(phone, message, response_text, new_stage, user_name)
            
        # ============= SALUDO INICIAL =============
        elif msg in ['hola', 'hi', 'hello', 'buenos dias', 'buenas tardes', 'buenas noches', 'buenas']:
            if not user_name:
                response_text = """¡Hola! 💕 Soy Ana de la Escuela de Joyería Tejida del Perú ✨

Para conocerte mejor, ¿cuál es tu nombre?"""
                new_stage = 'ESPERANDO_NOMBRE'
            else:
                response_text = f"""¡Hola *{user_name}*! 💕 Qué gusto verte de nuevo!

¿En qué puedo ayudarte hoy?

*1* → Información de cursos desde cero
*2* → Cursos avanzados
*PRECIO* → Ver costos y promociones
*HORARIOS* → Ver calendario disponible
*AYUDA* → Hablar con una asesora"""
                new_stage = 'MENU_PRINCIPAL'
        
        # ============= CONFIRMACIÓN DE PAGO =============
        elif 'pagado' in msg or 'pague' in msg or 'pagué' in msg or 'ya pague' in msg or 'listo' in msg or 'realizado' in msg or 'hecho' in msg:
            nombre_display = user_name if user_name else "amig@"
            response_text = f"""🎉 *¡FELICITACIONES {nombre_display.upper()}!* 🎉

*Tu matrícula ha sido registrada exitosamente*

✅ Recibirás un mensaje de confirmación en las próximas horas
✅ Serás agregad@ al grupo VIP de WhatsApp
✅ Magally te dará la bienvenida personalmente

*Próximos pasos:*
1. Revisa tu WhatsApp para la invitación al grupo
2. Prepara tus materiales (lista en el grupo)
3. Inicio de clases: 15 de Abril

*¿Tienes alguna pregunta?* 
Escribe *AYUDA* para hablar con una asesora

¡Bienvenid@ a la familia de Joyería Tejida! 💕"""
            new_stage = 'PAGO_CONFIRMADO'
            
        # ============= OPCIÓN 1: CURSO BÁSICO =============
        elif msg == '1' or 'desde cero' in msg or 'principiante' in msg:
            nombre_display = user_name if user_name else ""
            response_text = f"""*🌸 CURSO INTEGRAL - DESDE CERO*

Perfect{' ' + nombre_display if nombre_display else ''}! Te recomiendo nuestro Curso Integral 👑

*Incluye:*
✔️ 3 niveles: básico, intermedio y avanzado
✔️ 6 meses de formación completa
✔️ No requiere experiencia previa
✔️ Materiales incluidos (envío en Perú)
✔️ 6 clases en vivo + 60 horas práctica

*Precio especial:* USD 182 (regular USD 192)
*Separa tu lugar:* Solo S/100

¿Quieres más información?
*PRECIO* → Ver formas de pago
*HORARIOS* → Ver calendario
*MATRICULA* → Inscribirte ahora"""
            new_stage = 'INTERESADO_BASICO'
        
        # ============= OPCIÓN 2: CURSOS AVANZADOS =============
        elif msg == '2' or 'avanzado' in msg or 'experiencia' in msg:
            response_text = """*💎 CURSOS AVANZADOS*

¡Genial! Para perfeccionar tu técnica tenemos:

*Técnicas Especializadas:*
- Engaste de piedras preciosas
- Filigrana avanzada
- Diseño 3D en joyería

*Colecciones Premium:*
- Joyería nupcial
- Línea masculina
- Alta joyería

*Business & Marca:*
- Marketing para joyeros
- Creación de marca personal

Escribe:
*CATALOGO* → Ver todos los cursos
*PRECIO* → Ver costos
*HORARIOS* → Ver calendario"""
            new_stage = 'INTERESADO_AVANZADO'
        
        # ============= PRECIOS =============
        elif 'precio' in msg or 'costo' in msg or 'cuanto' in msg or 'valor' in msg:
            response_text = """*💎 PRECIOS Y PROMOCIÓN ESPECIAL*

*Curso por Nivel:* USD 192
*🔥 Con descuento:* USD 182 (hasta el 14 de abril)

✅ Separa tu matrícula con solo S/100
✅ Pago en 2 cuotas disponible
✅ Materiales GRATIS con entrega

*FORMAS DE PAGO:*
📱 *Yape:* 989 982 183
🏦 *BCP:* 192-16056570056
A nombre de: Magally Juro Huerta

Escribe *MATRICULA* para inscribirte ahora"""
            new_stage = 'CONSULTO_PRECIO'
        
        # ============= MATRÍCULA =============
        elif 'matricula' in msg or 'inscrib' in msg or 'pagar' in msg or 'yape' in msg:
            response_text = """*💳 PROCESO DE MATRÍCULA*

*Paso 1:* Realiza el pago
📱 *Yape:* 989 982 183
🏦 *BCP:* 192-16056570056
Nombre: Magally Juro Huerta

*Paso 2:* Envía tu voucher aquí 📸

*Paso 3:* Te agregamos al grupo VIP donde Magally te da la bienvenida personalmente 🎉

¿Ya realizaste el pago? Envía tu voucher o escribe *PAGADO*"""
            new_stage = 'PROCESO_MATRICULA'
        
        # ============= HORARIOS =============
        elif 'horario' in msg or 'cuando' in msg or 'hora' in msg or 'calendario' in msg:
            response_text = """*📅 HORARIOS DISPONIBLES*

*🌅 MAÑANAS:* Lun y Mié 9-11am
*☀️ TARDES:* Mar y Jue 3-5pm
*🌙 NOCHES:* Mié y Vie 7-9pm
*📱 ONLINE:* Sábados 10am

Todos en hora Perú (GMT-5)
*Próximo inicio:* 15 de Abril

¿Cuál prefieres? Escribe:
*MAÑANAS* / *TARDES* / *NOCHES* / *ONLINE*"""
            new_stage = 'CONSULTO_HORARIOS'
        
        # ============= SELECCIÓN DE HORARIOS =============
        elif msg in ['online', 'sabado', 'sábado', 'sabados', 'sábados']:
            nombre_display = user_name if user_name else ""
            response_text = f"""*📱 HORARIO ONLINE SELECCIONADO*

Perfecto{' ' + nombre_display if nombre_display else ''}! Has elegido:

📅 *Sábados 10:00 AM* (Hora Perú)
💻 Clases por Zoom
📹 Grabaciones disponibles 1 semana
💬 Grupo WhatsApp exclusivo

Para reservar tu lugar escribe *MATRICULA*"""
            new_stage = 'SELECCIONO_HORARIO'
        
        # ============= AYUDA HUMANA =============
        elif msg in ['ayuda', 'asesora', 'asesor', 'humano', 'persona', 'hablar con alguien', 'duda', 'pregunta']:
            nombre_display = user_name if user_name else ""
            response_text = f"""*👩‍💼 CONTACTO CON ASESORA PERSONAL*

{nombre_display + ', t' if nombre_display else 'T'}e voy a conectar con una de nuestras asesoras especializadas.

*Horario de atención:*
- Lun-Vie: 9am - 7pm
- Sábados: 10am - 2pm

*Contacto directo:*
📱 WhatsApp: +51 989 982 183
📧 Email: info@joyeriatejida.com

_Tu consulta ha sido registrada. Una asesora te contactará pronto._

¿Es urgente? Escribe *URGENTE*"""
            new_stage = 'REQUIERE_ASESORA'
        
        # ============= VOLVER AL MENÚ =============
        elif msg in ['volver', 'menu', 'menú', 'inicio', 'regresar', 'atras', 'atrás']:
            nombre_display = user_name if user_name else 'Hola'
            response_text = f"""*📋 MENÚ PRINCIPAL*

{nombre_display}, ¿en qué puedo ayudarte?

*1* → Curso desde cero
*2* → Cursos avanzados
*PRECIO* → Ver precios
*HORARIOS* → Ver calendario
*MATRICULA* → Inscribirte
*AYUDA* → Hablar con asesora

¿Qué te gustaría saber?"""
            new_stage = 'MENU_PRINCIPAL'
        
        # ============= RESPUESTA POR DEFECTO =============
        else:
            # Si ya tiene nombre, usa menú personalizado
            if user_name:
                response_text = f"""No entendí tu mensaje 🤔

*{user_name}, ¿necesitas ayuda?*

*Opciones disponibles:*
*1* → Curso desde cero
*2* → Cursos avanzados
*PRECIO* → Ver precios
*HORARIOS* → Ver calendario
*MATRICULA* → Inscribirte
*AYUDA* → Hablar con asesora

Si tienes una duda específica, escribe *AYUDA* para hablar con una asesora personal."""
            else:
                response_text = """No entendí tu mensaje 🤔

*Opciones disponibles:*
*1* → Curso desde cero
*2* → Cursos avanzados
*PRECIO* → Ver precios
*HORARIOS* → Ver calendario
*MATRICULA* → Inscribirte
*AYUDA* → Hablar con asesora

¿En qué te puedo ayudar?"""
            new_stage = 'MENU_PRINCIPAL'
        
        # Guardar interacción (solo si no se guardó antes)
        if new_stage != 'NOMBRE_CAPTURADO':  # Ya se guardó arriba
            save_interaction(phone, message, response_text, new_stage, user_name)
        
        print(f"✅ Respuesta enviada. Stage: {current_stage} → {new_stage}")
        
        # Respuesta XML para Twilio
        xml_response = f"""<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Message>
        <Body>{response_text}</Body>
    </Message>
</Response>"""
        
        return {
            'statusCode': 200,
            'headers': {'Content-Type': 'application/xml'},
            'body': xml_response
        }
        
    except Exception as e:
        print(f"❌ ERROR: {str(e)}")
        import traceback
        traceback.print_exc()
        
        return {
            'statusCode': 200,
            'headers': {'Content-Type': 'application/xml'},
            'body': """<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Message>
        <Body>Disculpa, hubo un error. Por favor intenta de nuevo o escribe *AYUDA* para hablar con una asesora.</Body>
    </Message>
</Response>"""
        }