FROM public.ecr.aws/lambda/python:3.11

# Copiar archivos
COPY requirements.txt .
RUN pip install -r requirements.txt

COPY lambda_function.py .
COPY handlers/ ./handlers/
COPY templates/ ./templates/
COPY utils/ ./utils/

CMD ["lambda_function.lambda_handler"]