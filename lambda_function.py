import json
import base64
import boto3
from urllib.parse import parse_qs
from datetime import datetime
from decimal import Decimal

# Configuración
dynamodb = boto3.resource('dynamodb')
table = dynamodb.Table('chatbot-leads')

# Cargar templates
with open('responses.json', 'r', encoding='utf-8') as f:
    TEMPLATES = json.load(f)

# Etapas del embudo mejoradas
STAGES = {
    'NUEVO': 'Primer contacto',
    'ESPERANDO_NOMBRE': 'Pidiendo nombre',
    'NOMBRE_CAPTURADO': 'Nombre obtenido',
    'MENU_PRINCIPAL': 'En menú',
    'INTERESADO_BASICO': 'Curso básico',
    'INTERESADO_AVANZADO': 'Curso avanzado',
    'CONSULTO_ONLINE': 'Vio modalidad online',
    'CONSULTO_PRESENCIAL': 'Vio modalidad presencial',
    'CONSULTO_PRECIO': 'Vio precios',
    'CONSULTO_HORARIOS': 'Vio horarios',
    'SELECCIONO_HORARIO': 'Eligió horario',
    'CONSULTO_CATALOGO': 'Vio catálogo',
    'CURSO_CORAZON': 'Interesado en curso corazón',
    'CURSO_ANILLOS': 'Interesado en curso anillos',
    'PROCESO_MATRICULA': 'En matrícula',
    'ESPERANDO_VOUCHER': 'Esperando comprobante',
    'PAGO_CONFIRMADO': 'Pagó - Cliente',
    'REQUIERE_ASESORA': 'Necesita ayuda',
    'URGENTE_REQUIERE_ASESORA': 'Urgente - Atención prioritaria',
    'CONVERSACION_CERRADA': 'Conversación finalizada amablemente'
}

def get_user_data(phone):
    """Obtiene el historial completo del usuario"""
    try:
        response = table.query(
            KeyConditionExpression='phone_number = :phone',
            ExpressionAttributeValues={':phone': phone},
            ScanIndexForward=False
        )
        if response['Items']:
            # Buscar nombre en todo el historial
            user_name = None
            for item in response['Items']:
                if item.get('user_name'):
                    user_name = item.get('user_name')
                    break
            
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

def get_template_response(template_key, user_name=''):
    """Obtiene respuesta del template y reemplaza variables"""
    template = TEMPLATES['templates'].get(template_key, TEMPLATES['templates']['fallback'])
    text = template['text'].replace('{{name}}', user_name if user_name else 'Amig@')
    stage = template.get('stage', 'MENU_PRINCIPAL')
    return text, stage

def lambda_handler(event, context):
    try:
        # Decodificar mensaje
        body_str = event.get('body', '')
        if event.get('isBase64Encoded', False):
            body_str = base64.b64decode(body_str).decode('utf-8')
        
        body = parse_qs(body_str)
        message = body.get('Body', [''])[0]
        phone = body.get('From', [''])[0].replace('whatsapp:', '').replace('%3A', ':').replace('+', '')
        num_media = int(body.get('NumMedia', ['0'])[0])
        
        print(f"📱 Mensaje de {phone}: '{message}' | Media: {num_media}")
        
        msg = message.lower().strip()
        
        # Obtener datos previos
        user_data = get_user_data(phone)
        user_name = user_data.get('user_name', '')
        current_stage = user_data.get('stage', 'NUEVO')
        
        print(f"👤 Usuario: {user_name if user_name else 'Sin nombre'}, Stage: {current_stage}")
        
        response_text = ""
        new_stage = current_stage
        
        # ==== FLUJO PRINCIPAL DEL EMBUDO ====
        
        # 1. COMANDO RESET (oculto)
        if msg in ['reset', 'reiniciar', 'empezar de nuevo']:
            response_text, new_stage = get_template_response('reset_conversation')
            user_name = ''  # Limpiar nombre
        
        # 2. SALUDO INICIAL
        elif msg in ['hola', 'hi', 'hello', 'buenos dias', 'buenas tardes', 'buenas noches', 'buenas', 'hola!', 'ola']:
            if not user_name or user_name == 'Sin nombre':
                response_text, new_stage = get_template_response('waiting_name')
            else:
                response_text, new_stage = get_template_response('greeting_returning', user_name)
        
        # 3. CAPTURA DE NOMBRE
        elif current_stage == 'ESPERANDO_NOMBRE' and not user_name:
            user_name = message.strip().title()
            response_text, new_stage = get_template_response('greeting_with_name', user_name)
            save_interaction(phone, message, response_text, new_stage, user_name)
        
        # 4. DETECCIÓN DE VOUCHER/IMAGEN
        elif num_media > 0 or ('voucher' in msg and current_stage in ['PROCESO_MATRICULA', 'ESPERANDO_VOUCHER']):
            nombre_display = user_name if user_name else "amig@"
            response_text, new_stage = get_template_response('payment_confirmed', nombre_display)
        
        # 5. CONFIRMACIÓN VERBAL DE PAGO
        elif any(word in msg for word in ['pagado', 'pague', 'pagué', 'ya pague', 'listo', 'realizado', 'hecho']) and current_stage in ['PROCESO_MATRICULA', 'ESPERANDO_VOUCHER']:
            response_text, new_stage = get_template_response('waiting_voucher', user_name)
        
        # 6. SELECCIÓN DE CURSO BÁSICO
        elif msg == '1' or any(word in msg for word in ['desde cero', 'principiante', 'basico', 'básico']):
            response_text, new_stage = get_template_response('beginner_flow', user_name)
        
        # 7. SELECCIÓN DE CURSO AVANZADO
        elif msg == '2' or any(word in msg for word in ['avanzado', 'experiencia', 'intermedio']):
            response_text, new_stage = get_template_response('advanced_flow', user_name)
        
        # 8. MODALIDADES
        elif 'online' in msg and current_stage in ['INTERESADO_BASICO', 'MENU_PRINCIPAL', 'CONSULTO_HORARIOS']:
            response_text, new_stage = get_template_response('course_online_details', user_name)
        
        elif 'presencial' in msg and current_stage in ['INTERESADO_BASICO', 'MENU_PRINCIPAL', 'CONSULTO_HORARIOS']:
            response_text, new_stage = get_template_response('course_presencial_details', user_name)
        # 9. CURSOS AVANZADOS ESPECÍFICOS - VERSIÓN COMPLETA
        elif 'corazon' in msg or 'corazón' in msg:
            response_text = """*💝 CURSO JOYAS MODELO CORAZÓN*

Aprende a crear hermosas joyas con la forma más universal del amor.

*PROGRAMA COMPLETO:*
• Clase 1: Corazón Básico Tradicional
• Clase 2: Corazón Bombé Volumétrico  
• Clase 3: Corazón Helicoidal Avanzado
• Clase 4: Corazón con Textura Especial
• Clase 5: Corazón Doble Entrelazado
• Clase 6: Corazón Miniatura para Aretes
• Clase 7: Corazón XXL Statement
• Clase 8: Corazón con Incrustaciones

*INCLUYE:*
✅ 8 clases paso a paso
✅ Videos HD de alta calidad
✅ Materiales sugeridos
✅ Patrones y moldes
✅ Soporte técnico 24/7

*PRECIO NORMAL:* 98 USD
*🔥 PRECIO ESPECIAL:* 84 USD

*Inscripción directa:*
https://goe.run/curso-corazon-2024

*INSCRIBIR* → Confirmar inscripción
*VOLVER* → Ver otros cursos
*CATALOGO* → Ver todos los cursos"""
            new_stage = 'CURSO_CORAZON'
        
        elif 'anillos' in msg or 'anillo' in msg or 'festival' in msg:
            response_text = """*💍 FESTIVAL DE ANILLOS*

El curso más completo de anillos tejidos. ¡Domina todas las técnicas!

*PROGRAMA FESTIVAL:*
• Clase 1: Anillo Clásico Base
• Clase 2: Anillo con Piedra Central
• Clase 3: Anillo Helicoidal Elegante
• Clase 4: Anillo Bombé Voluminoso
• Clase 5: Anillo Trenzado Artesanal
• Clase 6: Anillo Minimalista Moderno
• Clase 7: Anillo Vintage Retro
• Clase 8: Anillo Compromiso Especial
• Clase 9: Set de 3 Anillos Apilables
• Clase 10: Anillo Ajustable Universal

*INCLUYE:*
✅ 10 clases magistrales
✅ Técnicas para todas las tallas
✅ Tips de acabados profesionales
✅ Herramientas recomendadas
✅ Certificado de especialización

*PRECIO NORMAL:* 128 USD
*🔥 PRECIO ESPECIAL:* 98 USD

*Inscripción directa:*
https://goe.run/festival-anillos-2024

*INSCRIBIR* → Confirmar inscripción
*VOLVER* → Ver otros cursos
*CATALOGO* → Ver todos los cursos"""
            new_stage = 'CURSO_ANILLOS'
        
        elif 'vibraciones' in msg or 'vibración' in msg:
            response_text = """*✨ CURSO JOYAS TEJIDAS VIBRACIONES*

Taller de 8 joyas avanzadas que te sorprenderán y movilizarán tu creatividad.

*PROGRAMA VIBRACIONAL:*
• Clase 1: Joya Malla Relámpago
• Clase 2: Joya Malla Pentágono Helicoidal
• Clase 3: Joya Patronaje Cuadrado
• Clase 4: Joya con Patronaje en Círculo
• Clase 5: Joya de Malla Multicalibrada
• Clase 6: Joya Malla Volumétrica
• Clase 7: Flor Helicoidal Especial
• Clase 8: Arete Destello Brillante

*TÉCNICAS EXCLUSIVAS:*
✅ Patronaje geométrico avanzado
✅ Mallas complejas profesionales
✅ Efectos visuales únicos
✅ Combinación de texturas
✅ Acabados de alta gama

*PRECIO NORMAL:* 112 USD
*🔥 PRECIO ESPECIAL:* 82 USD

Joyas entregadas por alumnas.
https://web.facebook.com/media/set/?set=a.1018857693582536&type=3&locale=es_LA


*Inscripción directa:*
https://goe.run/680daccd010e2

*INSCRIBIR* → Confirmar inscripción
*VOLVER* → Ver otros cursos
*CATALOGO* → Ver todos los cursos"""
            new_stage = 'CURSO_VIBRACIONES'
        
        elif 'mama' in msg or 'mamá' in msg or 'madre' in msg:
            response_text = """*💐 CURSO JOYAS PARA MAMÁ*

Aprenderás a elaborar 10 joyas hermosas más una de regalo especial.

*PROGRAMA MATERNAL:*
• Clase 1: Joya Medusa Elegante
• Clase 2: Joya Tulipán Primaveral
• Clase 3: Joya Tulipán con Recurso Chasis
• Clase 4: Joya Hespitan Helicoidal
• Clase 5: Joya Ameba Orgánica
• Clase 6: Joya Marinera Náutica
• Clase 7: Joya Corazón Bombé Helicoidal
• Clase 8: Orquídea Guaria de Costa Rica
• Clase 9: Joya Colección Madre/Hija
• Clase 10: Joya Arete Bombé Sofisticado
• Clase 11: REGALO - Joya Mágica Sorpresa

*IDEAL PARA:*
✅ Regalo perfecto para mamá
✅ Día de las madres
✅ Proyectos madre-hija
✅ Recuerdos familiares
✅ Emprendimiento maternal

*PRECIO NORMAL:* 120 USD
*🔥 PRECIO ESPECIAL:* 84 USD

Joyas entregadas por alumnas.
https://www.facebook.com/media/set/?set=a.1027447639390208&type=3&locale=es_LA&_rdc=1&_rdr#


*Inscripción directa:*
https://goe.run/641a560abf92a

*INSCRIBIR* → Confirmar inscripción
*VOLVER* → Ver otros cursos
*CATALOGO* → Ver todos los cursos"""
            new_stage = 'CURSO_MAMA'

        elif 'helicoidales' in msg or 'helicoidal' in msg:
            response_text = """*🌀 CURSO JOYAS HELICOIDALES*

Domina la técnica más elegante y sofisticada de la joyería tejida.

*PROGRAMA HELICOIDAL:*
• Clase 1: Fundamentos de la Técnica Helicoidal
• Clase 2: Helicoidal Simple Básico
• Clase 3: Helicoidal Doble Entrelazado
• Clase 4: Helicoidal Triple Complejo
• Clase 5: Helicoidal con Variaciones de Color
• Clase 6: Helicoidal Bombé Voluminoso
• Clase 7: Aretes Helicoidales Pareados
• Clase 8: Collar Helicoidal Largo
• Clase 9: Pulsera Helicoidal Ajustable

*TÉCNICAS AVANZADAS:*
✅ Matemática de la espiral perfecta
✅ Control de tensión especializado  
✅ Transiciones suaves de color
✅ Acabados profesionales
✅ Variaciones de grosor

*PRECIO NORMAL:* 108 USD
*🔥 PRECIO ESPECIAL:* 83 USD

Joyas entregadas por alumnas.
https://www.facebook.com/media/set/?set=a.1032230308911941&type=3&locale=es_LA&_rdc=1&_rdr#

*Inscripción directa:*
https://goe.run/curso-helicoidales-2024

*INSCRIBIR* → Confirmar inscripción
*VOLVER* → Ver otros cursos
*CATALOGO* → Ver todos los cursos"""
            new_stage = 'CURSO_HELICOIDALES'

        elif 'flores' in msg or 'flor' in msg or 'florales' in msg:
            response_text = """*🌸 CURSO FLORES PARA MAMÁ*

Crea las flores más hermosas y realistas en joyería tejida.

*PROGRAMA FLORAL:*
• Clase 1: Rosa Clásica Tradicional
• Clase 2: Tulipán Holandés Elegante
• Clase 3: Orquídea Tropical Exótica
• Clase 4: Girasol Radiante Alegre
• Clase 5: Margarita Campestre Dulce
• Clase 6: Lirio Imperial Majestuoso
• Clase 7: Gardenia Perfumada Refinada
• Clase 8: Violeta Africana Delicada
• Clase 9: Bouquet Combinado Especial
• Clase 10: Corona Floral Primaveral

*TÉCNICAS BOTÁNICAS:*
✅ Pétalos realistas en 3D
✅ Gradientes naturales de color
✅ Texturas orgánicas auténticas
✅ Ensamble de bouquets
✅ Follaje y complementos

*PRECIO NORMAL:* 125 USD
*🔥 PRECIO ESPECIAL:* 96 USD

*Inscripción directa:*
https://goe.run/curso-flores-mama-2024

*INSCRIBIR* → Confirmar inscripción
*VOLVER* → Ver otros cursos
*CATALOGO* → Ver todos los cursos"""
            new_stage = 'CURSO_FLORES'

        elif 'tiaras' in msg or 'tiara' in msg or 'tocados' in msg or 'tocado' in msg:
            response_text = """*👑 CURSO TIARAS Y TOCADOS*

El curso más exclusivo para crear tocados dignos de realeza.

*PROGRAMA REAL:*
• Clase 1: Tiara Clásica Princesa
• Clase 2: Corona Reina Majestuosa  
• Clase 3: Tocado Vintage Gatsby
• Clase 4: Diadema Griega Antigua
• Clase 5: Tocado Boho Silvestre
• Clase 6: Tiara Nupcial Elegante
• Clase 7: Corona Floral Primaveral
• Clase 8: Tocado Oriental Zen
• Clase 9: Tiara Gótica Dramática
• Clase 10: Corona de Quinceañera
• Clase 11: Tocado Carnaval Festivo
• Clase 12: Tiara Minimalista Moderna

*NIVEL PREMIUM:*
✅ Diseños de alta costura
✅ Técnicas de joyería fina
✅ Incrustación de cristales
✅ Estructuras arquitectónicas
✅ Acabados de lujo profesional

*PRECIO NORMAL:* 180 USD
*🔥 PRECIO ESPECIAL:* 139 USD

Joyas entregadas por nuestras alumnas:
https://www.facebook.com/media/set/?set=a.346272843740436&type=3&locale=es_LA&_rdc=1&_rdr#

*Inscripción directa:*
https://goe.run/curso-tiaras-tocados-2024

*INSCRIBIR* → Confirmar inscripción
*VOLVER* → Ver otros cursos
*CATALOGO* → Ver todos los cursos"""
            new_stage = 'CURSO_TIARAS'

        # 12. CATÁLOGO MEJORADO
        elif any(word in msg for word in ['catalogo', 'catálogo', 'todos', 'cursos']):
            response_text = """*📚 CATÁLOGO COMPLETO DE CURSOS AVANZADOS*

Elige el curso que más despierte tu creatividad:

*💝 CORAZON* → Joyas Modelo Corazón ($84)
💍 *ANILLOS* → Festival de Anillos ($98)
✨ *VIBRACIONES* → Joyas Tejidas Vibraciones ($82)
💐 *MAMA* → Joyas para Mamá ($84)
🌀 *HELICOIDALES* → Joyas Helicoidales ($83)
🌸 *FLORES* → Flores para Mamá ($96)
👑 *TIARAS* → Tiaras y Tocados ($139)

*INFORMACIÓN ADICIONAL:*
*PRECIOS* → Ver todos los precios
*HORARIOS* → Consultar horarios disponibles
*ONLINE* → Modalidad virtual
*PRESENCIAL* → Modalidad presencial

Escribe el nombre del curso que más te interese para ver el programa completo."""
            new_stage = 'CONSULTO_CATALOGO'

        
        # 10. CONSULTA DE PRECIOS
        elif any(word in msg for word in ['precio', 'costo', 'cuanto', 'valor', 'pago']):
            if current_stage in ['INTERESADO_BASICO', 'CONSULTO_ONLINE', 'CONSULTO_PRESENCIAL']:
                response_text, new_stage = get_template_response('pricing_integral', user_name)
            else:
                response_text, new_stage = get_template_response('pricing_integral', user_name)
        
        # 11. PROCESO DE MATRÍCULA
        elif any(word in msg for word in ['matricula', 'inscrib', 'inscripcion', 'inscribir','matrícula','Matricula','Matrícula']):
            response_text, new_stage = get_template_response('payment_process', user_name)
        
        # 12. CATÁLOGO
        elif any(word in msg for word in ['catalogo', 'catálogo', 'todos']):
            response_text = """*📚 CATÁLOGO DE CURSOS AVANZADOS*

Elige el curso que más te interese:

*CORAZON* → Joyas Modelo Corazón ($84)
*ANILLOS* → Festival de Anillos ($98)
*VIBRACIONES* → Joyas Tejidas Vibraciones ($82)
*MAMA* → Joyas para Mamá ($84)
*HELICOIDALES* → Joyas Helicoidales ($83)
*FLORES* → Flores para Mamá ($96)
*TIARAS* → Tiaras y Tocados ($139)

Escribe el nombre del curso que te interesa."""
            new_stage = 'CONSULTO_CATALOGO'
        
        # 13. HORARIOS - AGREGADO
        elif any(word in msg for word in ['horario', 'horarios', 'cuando', 'hora', 'calendario', 'schedule']):
            response_text, new_stage = get_template_response('schedule_info', user_name)
        
        # 14. AYUDA
        elif any(word in msg for word in ['ayuda', 'asesora', 'asesor', 'persona', 'duda', 'pregunta', 'contacto']):
            response_text, new_stage = get_template_response('human_help', user_name)
        
        elif any(word in msg for word in ['urgente', 'urge', 'emergencia']):
            response_text = """*🚨 ATENCIÓN PRIORITARIA*

Tu caso ha sido marcado como URGENTE.

*Contacto inmediato:*
📱 *Llama ahora:* +51 989 982 183
📱 *WhatsApp Escuela:* +51 923 845 189

Una asesora te contactará en los próximos 30 minutos."""
            new_stage = 'URGENTE_REQUIERE_ASESORA'
        
        # 15. VOLVER AL MENÚ
        elif any(word in msg for word in ['volver', 'menu', 'menú', 'inicio', 'regresar', 'atras']):
            response_text, new_stage = get_template_response('greeting_returning', user_name)
        
        # 16. RESPUESTAS DE CIERRE
        elif any(word in msg for word in ['no gracias', 'no necesito', 'nada mas', 'nada más', 'eso es todo', 'listo gracias']):
            response_text, new_stage = get_template_response('conversation_closed', user_name)
        
        elif msg in ['gracias', 'ok', 'bien', 'perfecto', 'genial', 'excelente']:
            if current_stage in ['PAGO_CONFIRMADO', 'URGENTE_REQUIERE_ASESORA']:
                response_text = """¡De nada! 💕 
Estamos aquí para apoyarte en tu camino con la joyería tejida.
¿Hay algo más en lo que pueda ayudarte?"""
            elif current_stage == 'CONVERSACION_CERRADA':
                response_text = """¡Hasta pronto! 💕"""
            else:
                response_text = TEMPLATES['quick_responses'].get(msg, 
                    "¡Perfecto! ¿Hay algo más que quieras saber?")
            new_stage = current_stage
        
        elif msg in ['no', 'nada', 'no necesito'] and current_stage != 'ESPERANDO_NOMBRE':
            response_text = """Entiendo! Si necesitas algo más, aquí estaré 😊

Puedes escribir:
*HOLA* → Para volver a empezar
*AYUDA* → Para hablar con una asesora

¡Que tengas un lindo día!"""
            new_stage = 'CONVERSACION_CERRADA'
        
        elif msg in ['adios', 'bye', 'hasta luego', 'chau', 'nos vemos']:
            response_text, new_stage = get_template_response('conversation_closed', user_name)
        
        # 17. RESPUESTAS RÁPIDAS (sin conflictos)
        elif msg in TEMPLATES['quick_responses'] and msg not in ['gracias', 'ok', 'no', 'adios', 'bye']:
            response_text = TEMPLATES['quick_responses'][msg]
            new_stage = current_stage
        
        # 18. FALLBACK
        else:
            response_text, new_stage = get_template_response('fallback', user_name)
        
        # Guardar interacción (si no se guardó antes)
        if new_stage != 'NOMBRE_CAPTURADO':
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