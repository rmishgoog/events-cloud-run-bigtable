### Ingest timeseries event data into Cloud BigTable with Google Cloud Pub/Sub and Google Cloud Run

*Google Cloud BigTable is Google's sparsely populated database which can scale to billions of rows and thousands of columns, enabling you to store terabytes or even petabytes of data at scale. BigTable is a "NoSQL", key-value store which is ideal for storing large amount of "single-key" data, supporting a high throughput at low read/wrire latencies. A BigTable has only one index, that is the row key which will be used to look up the data, there are plenty of guidelines on how you should choose your row keys and design your schema, but broadly speaking, consider your query patterns for the store data and that should be the first thing that should guide your row keys and schema. Please refer to Google's documentation for more insights into BigTable design considerations.*

https://cloud.google.com/bigtable/docs/overview

*BigTable's unique backend architecture gives it many competitive advantages such as:*

- *Incredibly high scalability.*
- *No server management for you as a user, BigTable is fully managed by Google.*
- *Autoscaling.*
- *Global footprints, you can have big table clusters in one or more unique zones in a region or even in another zone in another region, all under a single BigTable instance,replication starts automatically without any extra configurations, making your data available across the geographies.*
- *Resize your clusters without any downtime.*
- *Simple administration, all updates, upgrades and restarts are hadnled seamlessly, ensuring data durability during such events.*

*BigTable makes a great choice for storing a variety of data:*
- *Time-series data*
- *Marketing data*
- *Financial data such as stock prices and currency exchange rates*
- *IoT*
- *Graph data, showing relationship between entities*

*In this tutorial we will use BigTable to implement a time-series database, where events from an external source arrive at regular intervals of time, are ingested into Cloud Pub/Sub and pushed to a Cloud Run API which then handles the events and persist the data into BigTable.*

*Our tutorial simulates a climate/weather monitoring office, they get events through some sensors placed in several big cities and towns across the United States, with each event describing the climatic condition at a given point in time, such as temperature, air pressure, pollution score etc. These sensors emit these events several times (per-minute) during the course of a week and gets logged into BigTable. At the end of the week, the designated 'climate watchdog' can use this data for several analytical and other use cases.*

*In this time bucket pattern, you add new cells to existing columns when you write a new event. This pattern lets you take advantage of Bigtable's ability to let you store multiple timestamped cells in a given row and column. It's important to specify garbage collection rules when you use this pattern. The commands that I have included will show how do you do that when you create a table via the CLI.*

*Also, pay attention to the row key that will be used, it wil be of the format state#county#city#week, where # is the de-limiter, this row key will act as an identifier of the week's data. At the end of the week, each column in each row has one measurement for each minute of the week, or 10,080 cells (if your garbage collection policy allows it).*

*Let's begin provisioning some stuff and see our application/API in action!*

*First, make sure that you have an active GCP project with a valid billing account, also ensure that you have right roles and permissions in order to provision the resources we will need for this tutorial. For the sake of simplicity, use an account with 'Project Owner' or 'Project Editor' role assigned to it, however, always keep in mind that convinience over security is never a good practice and I highly recommend that in your production environments, always use the principle of least privileges, assigning the identities exactly what they need, no more and no less.*

*On your workstation, if you do not have the gcloud CLI installed, please go ahead and install it, if you do, you can move on to the next step.*
```
gcloud auth login
```
*Follow the web flow and authenticate yourself with the 'owner/editor' account that you are using, this command will obtain access credentials for your user account via a web-based authorization flow. When this command completes successfully, it sets the active account in the current configuration to the account specified.*

*Next, execute the below command:*
```
gcloud auth application-default login
```
*Follow the authentication flow when prompted.This command obtains user access credentials via a web flow and puts them in the well-known location for Application Default Credentials (ADC).This command is useful when you are developing code that would normally use a service account but need to run the code in a local development environment where it's easier to provide user credentials. The credentials will apply to all API calls that make use of the Application Default Credentials client library.*

*So, why did we do both?*

*It's important to understand the differences, gcloud auth login will store the creds (credentials) at a known location which will be used by gcloud CLI, any code/SDK running locally will not pick up these creds, however as you would see down the line, we will be running some test clients which will make use of Google's client libraries and these libraries make use of ADC which iteratively look for creds to use when authenticating to Google Cloud APIs, the gcloud auth application-default login will update the creds at a location known to client libraries and allowing them to use the creds. Same holds good for tools like Terraform for example, they also rely on ADC rather gcloud and should you choose to run Terraform CLI locally with ADC supplied credentials, you must provide them, credentials update by gcloud auth application-defual login are picked in the last iteration. They do not overwrite what is written by gcloud auth login command.*

*Enable the required services if not done already:*
```
gcloud services enable run.googleapis.com \
  bigtable.googleapis.com \
  pubsub.googleapis.com \
  containerregistry.googleapis.com
```

*Find more details here https://cloud.google.com/sdk/gcloud/reference/auth/application-default/login*

*Clone the repo on your workstation:*
```
git clone https://github.com/rmishgoog/events-cloud-run-bigtable.git
```
*Change to teh application directory:*
```
cd events-cloud-run-bigtable/application
```
*Build the application image (you should have docker installed on your terminal or if you prefer to work with another tool such as podman, that will do as well, I have docker CLI and docker daemon running locally and thus using the same), take a look at the Dockerfile and how it makes use of multi-staged build to finally buld a minimalistic image with distroless at it's base and contaning just the go binary, a practice you shall follow when possible:*

***Note: Make sure you replace YOUR_PROJECT_ID with the project id you are working within, alternatively you can set PROJECT_ID as an environment variable and use it, for example export PROJECT_ID=my-project and then replace <YOUR_PROJECT_ID> with $PROJECT_ID**.*
```
docker build -t gcr.io/<YOUR_PROJECT_ID>/climate-updates:v0.1 .
```
*Now push this image to your Google Container Registry:*
```
docker push gcr.io/<YOUR_PROJECT_ID>/climate-updates:v0.1
```
*Create Cloud BigTable instance with a single cluster (we will use cbt as the CLI to interact with BigTable APIs, if you do not have it installed, you can easily do it via gcloud by following the next two instructions, before you create your instance):*
```
gcloud components update
```
```
gcloud components install cbt
```
```
cbt createinstance climate-updates "climate updates" climate-updates-c1 us-central1-a 1 SSD
```
*For convinience, let's update the local .cbt file where we can set the project id and instance as defaults to be used by future cbt commands:*
```
export \
    INSTANCE_ID=climate-updates
export \
    GOOGLE_CLOUD_PROJECT=<YOUR_PROJECT_ID>
echo project = \
    $GOOGLE_CLOUD_PROJECT > ~/.cbtrc
echo instance = $INSTANCE_ID >> \
    ~/.cbtrc
```
*Create  the table:*
```
cbt createtable climate-updates "families=climate_summary:maxage=10d||maxversions=1,stats_detail:maxage=10d||maxversions=1"
```
*Our application will be deployed as a Cloud Run container and to be able to read/write from BigTable, we should use a 'purposed' service account and grant it the role it needs:*
```
gcloud iam service-accounts create climate-updates-api --display-name="Cloud Run API service account"
```
```
gcloud projects add-iam-policy-binding rmishra-kubernetes-playground \
    --member=serviceAccount:climate-updates-api@<YOUR_PROJECT_ID>.iam.gserviceaccount.com --role=roles/bigtable.user
```
*Now, go ahead and deploy the Cloud Run service:*
```
gcloud run deploy climate-updates-ingest-api --image=gcr.io/<YOUR_PROJECT_ID>/climate-updates:v0.1 --concurrency=20 --cpu=1 --ingress=internal --memory=256Mi --port=8080 \
  --set-env-vars=PROJECT=<YOUR_PROJECT_ID>,INSTANCE=climate-updates,TABLE=climate-updates --no-allow-unauthenticated --execution-environment=gen1 --region=us-central1 \
  --service-account=climate-updates-api@<YOUR_PROJECT_ID>.iam.gserviceaccount.com
```
*Note down the service endpoint, we will use that when creating a 'push' subscription to automatically invoke the Cloud Run service when an event arrives at the topic. It should look similar to what is given below:*
```
https://climate-updates-ingest-api-7ljpzipopq-uc.a.run.app/
```
*Now, we move on to create a topic and a 'push' subscription, but before that, let's create a service account which the subscripton can use to authenticate itself when invoking the Cloud Run service:*
```
gcloud iam service-accounts create climate-updates-subscription
```
*Grant this service account the run.invoker role on the Cloud Run service we created above:*
```
gcloud run services add-iam-policy-binding climate-updates-ingest-api  --member=serviceAccount:climate-updates-subscription@<YOUR_PROJECT_ID>.iam.gserviceaccount.com  --role=roles/run.invoker
```
*Create a pub/sub topic, this is where our "sensors" will post the climate condition updates:*
```
gcloud pubsub topics create us-climate-updates
```
*Finally, create the subscription, associate it with the topic and make sure you assign it the right service account (created above with run.invoker role), specify the type as 'push' and supply the Cloud Run service endpoint to post the message payload:*
```
gcloud pubsub subscriptions create us-climate-updates-subscription --topic us-climate-updates \
   --ack-deadline=60 \
   --push-endpoint=<YOUR_CLOUD_RUN_SERVICE_ENDPOINT> \
   --push-auth-service-account=climate-updates-subscription@<YOUR_PROJECT_ID>.iam.gserviceaccount.com \
   --min-retry-delay=60 \
   --max-retry-delay=600
 ```
*That should be it! We have provisioned all the GCP resources we need, you can do a spot check in GCP console that everything looks alright and is in a healthy state.*

*Next, we will test the set up with a simple standalone Go client.You will need Go installed on your workstation to test this set up, I will also update this section with a gcloud command to post complex messeges (like having JSON formatted payload instead of plain strings). If you have Go installed, change to the client directory:*
```
cd ../client
```
*Here, isnide the main method, be sure to supply the project id and topic id you are working with:*
```
func main() {
	fmt.Printf("main(). Starting to publish the message")
	//projectID := "<your-project-id"
	//topicID := "<your-topic-id>"
  ....
  ....
```
*The sample payload is a Go struct which is marshalled into a Json, you can change the values as you like and run the code, just make sure that values do not violate the declared types:*
```
data := PayLoad{
		State:          "IL",
		County:         "Will",
		City:           "Bolingbrook",
		PollutionIndex: 100,
		Temperature:    40.6,
		AirPressure:    30,
		WeekOfYear:     1,
		Year:           2023,
	}
```
*Run the Go client:*
```
go run .
```
*You should get a message like below when the client is finished running successfully:*
```
main(). Starting to publish the message{"State":"IL","County":"Will","City":"Bolingbrook","PollutionIndex":100,"Temperature":20.6,"AirPressure":30,"WeekOfYear":2,"Year":2023}
Published the message successfully with the id: 6569652341950790
Published the message successfully
```
*You can check the Cloud Run service logs additionally to make sure that there were no errors, let's now check if the entries made it to the Cloud BigTable, pay attention to the row keys, they are of the form State#County#City#Year#Week with '#' as the key delimiter, you can provide the values you use in the client code:*
```
cbt lookup climate-updates IL#Will#Bolingbrook#2023#2 columns=climate_summary:pollution,temperature,pressure
```
*And we have results:*
```
IL#Will#Bolingbrook#2023#2
  climate_summary:pollution                @ 2023/01/05-12:25:18.462000
    "SEVERE"
  climate_summary:pressure                 @ 2023/01/05-12:25:18.462000
    "30.000000"
  climate_summary:temperature              @ 2023/01/05-12:25:18.462000
```
*Note the timestamp here! Each column will have timestamped cells in our case, as stated, the 'sensors' emit this every minute throughout the week before flipping over to the next week. You should also make sure to read Google Cloud documentation on designing BigTable keys and schema which will make you understand better as why my row keys are of this form, in general you shall try to keep more granular values towards the end and more commmon values towards the begining of your row keys, for example you can have another city in the same state and county emitting the 'sensor' data, having row key as State#County#City#Year#Week is a good choice here.*

*Now, let's update the temperature reading and run the client another time, we can see how pollution, air pressure and temperatures are captured as timestamped data:*
```
IL#Will#Bolingbrook#2023#2
  climate_summary:pollution                @ 2023/01/05-12:40:11.434000
    "SEVERE"
  climate_summary:pollution                @ 2023/01/05-12:25:18.462000
    "SEVERE"
  climate_summary:pressure                 @ 2023/01/05-12:40:11.434000
    "30.000000"
  climate_summary:pressure                 @ 2023/01/05-12:25:18.462000
    "30.000000"
  climate_summary:temperature              @ 2023/01/05-12:40:11.434000
    "21.600000"
  climate_summary:temperature              @ 2023/01/05-12:25:18.462000
    "20.600000"
 ```
 *That's it! We just implemented a time-series "climate updates" ingestion system with "zero infrastructure" or "serverless" cloud native tech stack on Google Cloud, using Pub/Sub, Cloud Run and BigTable! How cool is that? Serverless takes the operational overheads away from your development teams so that they can focus on what they are measured against, that is to write quality applications while your provider like Google Cloud manages the underlying infrastructure for you.*
 
*Cleaning up the resources is equally important after you are done with the tutorial, you can simply delete the project if it was created for this purpose alone but if not, use the below commands to clean up the resources:*
```
cbt deleteinstance climate-updates
```
```
gcloud pubsub subscriptions delete us-climate-updates-subscription
```
```
gcloud pubsub topics delete us-climate-updates
```
```
gcloud run services delete climate-updates-ingest-api
```





