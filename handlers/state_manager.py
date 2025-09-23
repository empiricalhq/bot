# handlers/state_manager.py
import boto3
import json
from datetime import datetime, timedelta
from typing import Dict, List, Optional
from decimal import Decimal

class StateManager:
    def __init__(self, table, phone_number: str):
        self.table = table
        self.phone_number = phone_number
        self.conversation_ttl = 30 * 24 * 60 * 60  # 30 días en segundos
        
    def get_history(self, limit: int = 10) -> List[Dict]:
        """Obtiene el historial de conversación reciente"""
        try:
            # Query últimas N interacciones
            response = self.table.query(
                KeyConditionExpression='phone_number = :phone',
                ExpressionAttributeValues={
                    ':phone': self.phone_number
                },
                ScanIndexForward=False,  # Orden descendente (más reciente primero)
                Limit=limit
            )
            
            # Revertir para tener orden cronológico
            items = response.get('Items', [])
            return list(reversed(items))
            
        except Exception as e:
            print(f"Error getting history: {e}")
            return []
    
    def save_interaction(self, user_message: str, bot_response: str, 
                        metadata: Optional[Dict] = None) -> bool:
        """Guarda una interacción en la conversación"""
        try:
            timestamp = int(datetime.now().timestamp())
            ttl = timestamp + self.conversation_ttl
            
            item = {
                'phone_number': self.phone_number,
                'timestamp': timestamp,
                'user_message': user_message,
                'bot_response': bot_response,
                'ttl': ttl
            }
            
            if metadata:
                item['metadata'] = metadata
            
            # Convertir floats a Decimal para DynamoDB
            item = self._convert_floats_to_decimal(item)
            
            self.table.put_item(Item=item)
            return True
            
        except Exception as e:
            print(f"Error saving interaction: {e}")
            return False
    
    def get_conversation_context(self) -> Dict:
        """Obtiene el contexto actual de la conversación"""
        history = self.get_history()
        
        if not history:
            return {
                'is_new_user': True,
                'message_count': 0,
                'last_interaction': None,
                'user_data': {}
            }
        
        # Analizar el historial para extraer contexto
        context = {
            'is_new_user': False,
            'message_count': len(history),
            'last_interaction': history[-1] if history else None,
            'last_bot_message_type': None,
            'user_data': {}
        }
        
        # Extraer información del usuario del historial
        for interaction in history:
            metadata = interaction.get('metadata', {})
            
            # Guardar el tipo del último mensaje del bot
            if metadata.get('message_type'):
                context['last_bot_message_type'] = metadata['message_type']
            
            # Acumular datos del usuario
            if metadata.get('user_name'):
                context['user_data']['name'] = metadata['user_name']
            if metadata.get('course_interest'):
                context['user_data']['course_interest'] = metadata['course_interest']
            if metadata.get('experience_level'):
                context['user_data']['experience_level'] = metadata['experience_level']
        
        # Calcular tiempo desde última interacción
        if history:
            last_timestamp = history[-1].get('timestamp', 0)
            current_timestamp = datetime.now().timestamp()
            hours_since_last = (current_timestamp - last_timestamp) / 3600
            
            # Si han pasado más de 24 horas, es una nueva sesión
            context['is_new_session'] = hours_since_last > 24
        else:
            context['is_new_session'] = True
        
        return context
    
    def update_user_profile(self, profile_data: Dict) -> bool:
        """Actualiza el perfil del usuario"""
        try:
            # Guardar como una entrada especial de metadata
            timestamp = int(datetime.now().timestamp())
            
            item = {
                'phone_number': self.phone_number,
                'timestamp': timestamp,
                'user_message': '[PROFILE_UPDATE]',
                'bot_response': '[PROFILE_UPDATE]',
                'metadata': {
                    'type': 'profile_update',
                    'profile': profile_data
                },
                'ttl': timestamp + (365 * 24 * 60 * 60)  # 1 año para perfiles
            }
            
            item = self._convert_floats_to_decimal(item)
            self.table.put_item(Item=item)
            return True
            
        except Exception as e:
            print(f"Error updating profile: {e}")
            return False
    
    def get_user_profile(self) -> Dict:
        """Obtiene el perfil completo del usuario"""
        history = self.get_history(limit=50)  # Buscar en más historial
        
        profile = {
            'phone': self.phone_number,
            'name': None,
            'email': None,
            'course_interest': None,
            'experience_level': None,
            'preferred_schedule': None,
            'has_paid': False,
            'enrolled_courses': [],
            'interaction_count': len(history)
        }
        
        # Buscar datos del perfil en el historial
        for interaction in history:
            metadata = interaction.get('metadata', {})
            
            if metadata.get('type') == 'profile_update':
                # Actualización explícita del perfil
                profile.update(metadata.get('profile', {}))
            else:
                # Extraer datos de interacciones normales
                if metadata.get('user_name'):
                    profile['name'] = metadata['user_name']
                if metadata.get('user_email'):
                    profile['email'] = metadata['user_email']
                if metadata.get('course_interest'):
                    profile['course_interest'] = metadata['course_interest']
                if metadata.get('payment_confirmed'):
                    profile['has_paid'] = True
        
        return profile
    
    def mark_payment_received(self, payment_details: Dict) -> bool:
        """Marca que se recibió un pago del usuario"""
        try:
            metadata = {
                'type': 'payment_received',
                'payment_confirmed': True,
                'payment_details': payment_details,
                'payment_date': datetime.now().isoformat()
            }
            
            return self.save_interaction(
                user_message=f"[PAYMENT_RECEIVED: {payment_details.get('amount', 'N/A')}]",
                bot_response="¡Pago recibido! Te hemos agregado al grupo VIP 🎉",
                metadata=metadata
            )
        except Exception as e:
            print(f"Error marking payment: {e}")
            return False
    
    def get_conversation_summary(self) -> str:
        """Genera un resumen de la conversación para el LLM"""
        history = self.get_history()
        
        if not history:
            return "Nueva conversación, sin historial previo."
        
        summary_parts = []
        
        # Últimas 5 interacciones
        recent_history = history[-5:]
        for interaction in recent_history:
            user_msg = interaction.get('user_message', '')
            bot_msg = interaction.get('bot_response', '')
            
            if user_msg not in ['[PROFILE_UPDATE]', '[PAYMENT_RECEIVED]']:
                summary_parts.append(f"Usuario: {user_msg[:100]}")
                summary_parts.append(f"Bot: {bot_msg[:100]}")
        
        return "\n".join(summary_parts[-10:])  # Máximo 10 líneas
    
    def _convert_floats_to_decimal(self, obj):
        """Convierte floats a Decimal para DynamoDB"""
        if isinstance(obj, float):
            return Decimal(str(obj))
        elif isinstance(obj, dict):
            return {k: self._convert_floats_to_decimal(v) for k, v in obj.items()}
        elif isinstance(obj, list):
            return [self._convert_floats_to_decimal(item) for item in obj]
        return obj