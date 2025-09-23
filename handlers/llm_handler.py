# handlers/llm_handler.py
import boto3
import json

class LLMHandler:
    def __init__(self):
        self.bedrock = boto3.client('bedrock-runtime')
        
    def get_smart_response(self, message, context):
        # Primero intentar clasificar con Titan Express (baratísimo)
        classification = self.classify_intent(message)
        
        if classification == "OFF_TOPIC":
            return """Entiendo tu consulta 😊 En la Escuela de Magally nos especializamos 
                     en joyería tejida. ¿Te gustaría conocer nuestros cursos?"""
        
        elif classification == "COMPLEX":
            # Solo aquí usamos un LLM más potente
            return self.generate_response(message, context)
        
        # Default fallback
        return """No estoy segura de entender tu consulta. 
                 ¿Buscas información sobre:
                 • Cursos desde cero
                 • Cursos avanzados
                 • Precios y promociones
                 • Proceso de matrícula"""
    
    def classify_intent(self, message):
        # Usar Amazon Titan Express - super barato
        prompt = f"""Clasifica este mensaje en una categoría:
        Mensaje: {message}
        
        Categorías: CURSO_INFO, PRECIO, MATRICULA, OFF_TOPIC, COMPLEX
        Responde solo con la categoría."""
        
        body = json.dumps({
            "inputText": prompt,
            "textGenerationConfig": {
                "maxTokenCount": 10,
                "temperature": 0
            }
        })
        
        response = self.bedrock.invoke_model(
            modelId='amazon.titan-text-express-v1',
            body=body
        )
        
        return json.loads(response['body'].read())['results'][0]['outputText'].strip()