@Library('homelab-jenkins-library@main') _

pipeline {
    agent {
        kubernetes {
            yaml homelab.podTemplate('default')
        }
    }

    stages {
        stage('Build') {
            steps {
                echo 'Building...'
            }
        }

        stage('Test') {
            steps {
                echo 'Testing...'
            }
        }
    }

    post {
        failure {
            script {
                homelab.notifyDiscord(status: 'FAILURE')
            }
        }
    }
}
