# handlers/template_handler.py
import re
import json

class TemplateHandler:
    def __init__(self):
        self.load_templates()
        self.keywords = {
            r'\b(hola|buenos|buenas)\b': 'greeting',
            r'\b(precio|costo|cuanto|valor)\b': 'pricing',
            r'\b(desde cero|principiante|empezar|nuevo)\b': 'beginner',
            r'\b(experiencia|avanzado|intermedio)\b': 'advanced',
            r'\b(matricula|inscrib|registro)\b': 'enrollment',
            r'\b(pagar|yape|transferencia|bcp)\b': 'payment',
            r'\b(horario|cuando|dias|hora)\b': 'schedule',
            r'\b(certificado|diploma|titulo)\b': 'certification'
        }
    
    def load_templates(self):
        # En producción, esto vendría de S3 o DynamoDB
        self.templates = {
            'greeting': """Hola! 💕 Soy Ana, del área de soporte de la Escuela de Joyería Tejida 
                          
Te cuento que tenemos más de 15 años formando alumnas en toda América Latina 🌎

¿Ya sabes joyería tejida o deseas empezar desde cero?
- Escribe "1" para Desde Cero
- Escribe "2" si ya tienes experiencia""",
            
            'pricing': """💎 Nuestros precios:

Curso por Nivel: USD 192
Con promoción: USD 182 (hasta el 14 de abril)

✅ Puedes separar tu matrícula con solo S/100
✅ Aceptamos Yape, transferencia y pago en cuotas

¿Deseas información de matrícula? Escribe "matricula" """,
            
            'beginner': """Perfect! 🌸 Para empezar desde cero te recomiendo el Curso Integral

Incluye:
✔️ Tres niveles: básico, intermedio y avanzado
✔️ Online o presencial
✔️ No requiere experiencia
✔️ Materiales incluidos (Perú)
✔️ 6 clases en vivo + 60 horas práctica

¿Quieres ver los precios? Escribe "precio" """,
            
            'payment': """💳 Datos para matrícula:

Yape: 989 982 183
BCP Ahorro: 192-16056570056
A nombre de: Magally Juro Huerta

Una vez realizado el pago, envíanos el voucher y te agregamos al grupo! 🎉"""
        }
    
    def get_response(self, message, context):
        message_lower = message.lower()
        
        # Buscar coincidencias
        for pattern, template_key in self.keywords.items():
            if re.search(pattern, message_lower):
                return self.templates.get(template_key)
        
        # Respuestas numéricas (del menú)
        if message.strip() == "1":
            return self.templates['beginner']
        elif message.strip() == "2":
            return self.templates['advanced']
            
        return None