# utils/message_parser.py
import re
import json
from typing import Dict, Optional, Tuple

class MessageParser:
    def __init__(self):
        self.load_patterns()
        
    def load_patterns(self):
        """Carga patrones de responses.json"""
        with open('/var/task/templates/responses.json', 'r', encoding='utf-8') as f:
            data = json.load(f)
            self.keywords_map = data.get('keywords_map', {})
            self.quick_responses = data.get('quick_responses', {})
    
    def parse_message(self, message: str, context: Dict) -> Tuple[str, Optional[str], float]:
        """
        Analiza el mensaje y retorna:
        - intent: intención detectada
        - template_key: key de la plantilla a usar
        - confidence: confianza en la detección (0-1)
        """
        message_lower = message.lower().strip()
        
        # 1. Chequear respuestas numéricas del menú
        if message_lower in ['1', '2']:
            if context.get('last_bot_message_type') == 'greeting':
                return ('menu_selection', 
                       'beginner_flow' if message_lower == '1' else 'advanced_flow', 
                       1.0)
        
        # 2. Chequear respuestas rápidas exactas
        for quick_key, quick_response in self.quick_responses.items():
            if message_lower == quick_key:
                return ('quick_response', quick_key, 1.0)
        
        # 3. Buscar en patterns con regex
        best_match = None
        best_confidence = 0
        
        for pattern, template_key in self.keywords_map.items():
            if re.search(pattern, message_lower):
                # Calcular confianza basada en qué tan específico es el match
                words_matched = len(re.findall(pattern, message_lower))
                confidence = min(words_matched * 0.3, 1.0)
                
                if confidence > best_confidence:
                    best_confidence = confidence
                    best_match = template_key
        
        if best_match:
            return ('keyword_match', best_match, best_confidence)
        
        # 4. Analizar intención sin keywords específicos
        intent = self.analyze_intent(message_lower)
        
        return (intent, None, 0.3)
    
    def analyze_intent(self, message: str) -> str:
        """Análisis básico de intención cuando no hay match directo"""
        
        # Preguntas
        if any(q in message for q in ['?', 'que', 'como', 'cuando', 'donde', 'cual']):
            if any(word in message for word in ['curso', 'clase', 'aprender']):
                return 'question_about_courses'
            elif any(word in message for word in ['pagar', 'precio', 'costo']):
                return 'question_about_payment'
            else:
                return 'general_question'
        
        # Afirmaciones
        if any(word in message for word in ['quiero', 'necesito', 'busco', 'interesa']):
            return 'interested'
        
        # Off-topic común
        off_topic_patterns = [
            'clima', 'tiempo', 'noticias', 'futbol', 'politica',
            'receta', 'cocina', 'pelicula', 'musica'
        ]
        if any(pattern in message for pattern in off_topic_patterns):
            return 'off_topic'
        
        return 'unclear'
    
    def extract_entities(self, message: str) -> Dict:
        """Extrae entidades importantes del mensaje"""
        entities = {}
        
        # Detectar números de teléfono
        phone_pattern = r'\b\d{9,11}\b'
        phones = re.findall(phone_pattern, message)
        if phones:
            entities['phone'] = phones[0]
        
        # Detectar emails
        email_pattern = r'\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b'
        emails = re.findall(email_pattern, message)
        if emails:
            entities['email'] = emails[0]
        
        # Detectar nombres propios (básico)
        if 'me llamo' in message.lower() or 'mi nombre es' in message.lower():
            name_pattern = r'(?:me llamo|mi nombre es)\s+(\w+)'
            names = re.findall(name_pattern, message.lower())
            if names:
                entities['name'] = names[0].capitalize()
        
        # Detectar horarios preferidos
        if any(time in message.lower() for time in ['mañana', 'tarde', 'noche']):
            if 'mañana' in message.lower():
                entities['preferred_schedule'] = 'morning'
            elif 'tarde' in message.lower():
                entities['preferred_schedule'] = 'afternoon'
            elif 'noche' in message.lower():
                entities['preferred_schedule'] = 'evening'
        
        return entities
    
    def should_escalate_to_human(self, message: str, context: Dict) -> bool:
        """Determina si se debe escalar a un humano"""
        
        # Palabras que indican necesidad de atención humana
        escalation_triggers = [
            'hablar con alguien',
            'persona real',
            'humano',
            'urgente',
            'problema',
            'queja',
            'reclamo',
            'no funciona',
            'ayuda por favor'
        ]
        
        message_lower = message.lower()
        
        # Check escalation triggers
        if any(trigger in message_lower for trigger in escalation_triggers):
            return True
        
        # Si el usuario ha preguntado lo mismo 3 veces
        if context.get('repeated_intent_count', 0) >= 3:
            return True
        
        # Si la confianza es muy baja en múltiples mensajes
        if context.get('low_confidence_count', 0) >= 2:
            return True
        
        return False