.PHONY: dep build docker release install test backup

build:
	go build -o myapp

run: build
	./myapp

# docker:
# 	docker build -t ${G_SERVICE_NAME} .

run-docker: docker
	docker run --rm -e G_KEY -e G_PROJECT_ID -e PORT=8000 -p 8000:8000 ${G_SERVICE_NAME}

gbuild:
	# build on gcloud
	# gcloud builds submit -t gcr.io/${G_PROJECT_ID}/${G_SERVICE_NAME}
	# OR build locally and push
	# docker build -t gcr.io/${G_PROJECT_ID}/${G_SERVICE_NAME}:$GITHUB_SHA .
	docker build -t gcr.io/${G_PROJECT_ID}/${G_SERVICE_NAME}:latest .

deploy: gbuild
	# gcloud auth configure-docker
	docker push gcr.io/${G_PROJECT_ID}/${G_SERVICE_NAME}:latest
	gcloud run deploy ${G_SERVICE_NAME} \
            --region us-central1 \
            --image gcr.io/${G_PROJECT_ID}/${G_SERVICE_NAME}:latest \
            --platform managed \
            --allow-unauthenticated
