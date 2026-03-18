pipeline {
    agent any

    environment {
        // Define environment variables
        //DOCKER_IMAGE = 'sysadminaffiniti/affiniti_arm_ref'
        DOCKER_TAG = 'latest'
        DOCKER_IMAGE= 'affiniti-crm.duckdns.org:8080/affiniti_dms_finance_web:smp-drop8-latest'
        REGISTRY_URL  = "165.22.250.39:5000/${DOCKER_IMAGE}"
        REGISTRY_CREDENTIAL = 'docker_private' 
        GIT_CREDENTIAL = 'EpicGitLabNew'
        RECIPIENTS = "himan.a@affiniti.biz"
        GDRIVE_URL = "https://drive.google.com/drive/folders/1ihKlgIqPYIrvi1pWCaltJU9tRjaBIpK8?usp=sharing"
        //JAVA_HOME = '/opt/java/jdk-11.0.24'
        //PATH = "/opt/java/jdk-11.0.24/bin:/opt/maven/apache-maven-3.8.1/bin:${env.PATH}"

    }


    stages {

        stage('Checkout API Source') {
            steps {
                checkout([$class: 'GitSCM',
                    branches: [[name: 'client/sampath/drop_11/release/qa']],
                    userRemoteConfigs: [[
                        url: 'https://epictechdev.com:8081/affiniti_dms_project/affiniti_dms_finance_web.git',
                        credentialsId: env.GIT_CREDENTIAL
                    ]]
                ])
            }
        }

        stage('Build with Java 11') {
                    environment {
                        JAVA_HOME = '/opt/java/jdk-11.0.24'
                        PATH = "/opt/java/jdk-11.0.24/bin:${env.PATH}"
                    }
                    steps {
                        sh 'java -version' // should show Java 11
                        sh 'mvn  -version'
                       // sh 'mvn clean package jacoco:report '
                        sh 'mvn clean package -DskipTests '
                    }
                }
                
        stage('Build Docker Image') {
            steps {
                // Build Podman image
                sh "docker build -t ${DOCKER_IMAGE} ."
            }
        }


        stage('Copy Artifact and Docker Image to Remote Storage') {
            steps {
              script{
                env.dateOnly = sh(script: "date +%F", returnStdout: true).trim()             
                def timestamp = sh(script: "date +%F_%H-%M-%S", returnStdout: true).trim()   
                
                // Export environment variables so bash can see them
                env.artifactName = "AFFINITI_DMS_FINANCE_WEB_drop11-${timestamp}.war"
                env.TAR_FILE_NAME = "affiniti_dms_finance_web-${timestamp}.tar"
                
                // Rclone destination syntax (e.g. gdrive:Affinity/...)
                // IMPORTANT: Adjust 'gdrive:' if your remote is named differently
                env.RCLONE_DESTINATION = "gdrive:Affinity/Sampath/Deployments/${env.dateOnly}"
                env.RCLONE_TAR_TARGET = "${env.RCLONE_DESTINATION}/${env.TAR_FILE_NAME}"

                // Also store the mount path if we needed it, but CD agent will download to /tmp
                env.TAR_PATH = "/mnt/gdrive/Affinity/Sampath/Deployments/${env.dateOnly}/${env.TAR_FILE_NAME}"

                sh """
                # 1. Save Docker image to local temp file to prevent IO bottlenecks
                echo "⏳ Saving Docker image to tarball..."
                docker save ${DOCKER_IMAGE} -o /tmp/\${TAR_FILE_NAME}

                # 2. Upload WAR file directly using rclone copy for accurate progress tracking
                echo "⏳ Uploading WAR to Google Drive..."
                # Assuming the .war is inside the target directory
                # Note: We use the local filename parameter so it renames directly during upload
                rclone copyto target/*.war \${RCLONE_DESTINATION}/\${artifactName} --progress
                echo "✅ Uploaded WAR as \${artifactName}"

                # 3. Upload Tarball directly using rclone copy
                echo "⏳ Uploading Docker Tarball to Google Drive..."
                rclone copyto /tmp/\${TAR_FILE_NAME} \${RCLONE_TAR_TARGET} --progress
                echo "✅ Uploaded Tarball to \${RCLONE_TAR_TARGET}"
                
                # Cleanup
                rm /tmp/\${TAR_FILE_NAME}
                """
              }
          }
        }
        
        stage('Trigger CD Deployment Agent') {
            steps {
                script {
                    // Update this IP to point to the server where you started the Go CD Agent
                    def CD_AGENT_URL = "http://128.199.84.39:8080/api/v1/deploy"
                    def API_TOKEN = "60b334f36db720a2c3a602bade68ae438de0a76855fabd73021288d4d5265c3b" // From your config.yaml
                    
                    sh """
                    echo "🔗 Fetching Google Drive sharing link from rclone for \${RCLONE_TAR_TARGET}..."
                    GDRIVE_URL=\$(rclone link "\${RCLONE_TAR_TARGET}")
                    echo "Got URL: \${GDRIVE_URL}"
                    
                    echo "🧩 Extracting Google Drive File ID..."
                    # Extracts the ID directly from the /d/XXXXXX/view URL string
                    FILE_ID=\$(echo \$GDRIVE_URL | sed -n 's/.*id=\\([^&]*\\).*/\\1/p')
        
                    if [ -z "\$FILE_ID" ]; then
                      FILE_ID=\$(echo \$GDRIVE_URL | sed -n 's#.*/d/\\([^/]*\\).*#\\1#p')
                    fi
                    
                    echo "Extracted File ID: \${FILE_ID}"
                    
                    echo "🚀 Triggering deployment on CD Agent for project 'Sampath'..."
                    curl -X POST ${CD_AGENT_URL} \\
                      -H "Authorization: Bearer ${API_TOKEN}" \\
                      -H "Content-Type: application/json" \\
                      -d '{
                            "project": "Sampath",
                            "tar_path": "/opt/sampath/docker_images/${env.TAR_FILE_NAME}",
                            "gdrive_file_id": "'"\${FILE_ID}"'"
                          }'
                    """
                }
            }
        }



        stage('SonarQube Analysis (Java 17)') {
                    steps {
                        withSonarQubeEnv('MySonarQubeServer') {
                            sh 'java -version' // should show Java 17
                            sh '''
                            mvn org.sonarsource.scanner.maven:sonar-maven-plugin:4.0.0.4121:sonar \
                              -Dsonar.projectKey=Affinity_dms_finance_web \
                              -Dsonar.projectName=Affinity_dms_finance_web_drop11 \
                              -Dsonar.sources=src/main/java/com \
                              -Dsonar.exclusions=src/main/java/org/**/* \
                              -Dsonar.tests=src/test/java \
                              -Dsonar.java.binaries=target \
                              -Dsonar.coverage.jacoco.xmlReportPaths=target/site/jacoco/jacoco.xml \
                              -Dsonar.token=squ_8f73768bff896460eee71c297ac802c1e64bbeef
                            '''
                        }
                    }
                }

        stage('Quality Gate') {
            steps {
                timeout(time: 2, unit: 'MINUTES') {
                    waitForQualityGate abortPipeline: false
                }
            }
        }

        // stage('Publish Test Report') {
        //     steps {
        //         junit 'target/surefire-reports/*.xml'
        //     }
        // }
    }

    post {
        success {
            echo '✅ Build, test, and SonarQube analysis completed successfully!'
            
        }
        failure {
            echo '❌ Build or analysis failed.'
            
        }
    }
}
