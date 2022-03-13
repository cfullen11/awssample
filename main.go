package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

func main() {
	//Create a new key for infra admin account and set those creds
	ctx, nk := setInfraAdminAccess()
	setCreds(nk)

	//Get a list of ECR repos and create if does not exist
	ecrClient := getRepos(ctx)
	pushImage(ctx, ecrClient)
	
	//Create ECS if it does not exist
	createEcs(ctx)
}

func setInfraAdminAccess()(context.Context, *iam.CreateAccessKeyOutput) {
	userName := "infra-admin"
	maxItems := 10
	ctx := context.Background()
	// Load the Shared AWS Configuration (~/.aws/config)
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Create and iam client using the local config of the iam-admin
	client := iam.NewFromConfig(cfg)

	//List the existing keys of the infra admin user
	input := &iam.ListAccessKeysInput{
		MaxItems: aws.Int32(int32(maxItems)),
		UserName: &userName,
	}

	ak, err := client.ListAccessKeys(ctx, input)
	if err != nil {
		log.Fatal(err)
	}

	//For each existing key make inactive and then create a new key to be used for this session
	for _, key := range ak.AccessKeyMetadata {
		if key.Status == "Inactive" {
			_, err := client.DeleteAccessKey(ctx, &iam.DeleteAccessKeyInput{
				UserName: key.UserName,
				AccessKeyId: key.AccessKeyId,
			})
			if err != nil {
				log.Fatal(err)
			}
		}else{
			_, err := client.UpdateAccessKey(ctx, &iam.UpdateAccessKeyInput{
				UserName: key.UserName,
				AccessKeyId: key.AccessKeyId,
				Status: "Inactive",
			})
			if err != nil {
				log.Fatal(err)
			}
		}
	}

	nk, err := client.CreateAccessKey(ctx, &iam.CreateAccessKeyInput{
		UserName: &userName,
	})
	if err != nil {
		log.Fatal(err)
	}
	// defer client.UpdateAccessKey(ctx, &iam.UpdateAccessKeyInput{
	// 	UserName: nk.AccessKey.UserName,
	// 	AccessKeyId: nk.AccessKey.AccessKeyId,
	// 	Status: "Inactive",
	// })
	return ctx, nk
}

func setCreds(nk *iam.CreateAccessKeyOutput) {
	os.Setenv("AWS_ACCESS_KEY_ID", *nk.AccessKey.AccessKeyId)
	os.Setenv("AWS_SECRET_ACCESS_KEY", *nk.AccessKey.SecretAccessKey)
    os.Setenv("REGION", "us-east-2")
	time.Sleep(9 * time.Second )
}

func getEcrClient() *ecr.Client{
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil{
		log.Fatal(err)
	}
	client := ecr.NewFromConfig(cfg)
	return client
}

func getEcsClient() *ecs.Client{
	ctx := context.Background()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil{
		log.Fatal(err)
	}
	svcClient := ecs.NewFromConfig(cfg)
	return svcClient
}

func getRepos(ctx context.Context)*ecr.Client{
	client := getEcrClient()
	repoName := "skodaice"
	//var repos = []string {repoName}
	repodata, err := client.DescribeRepositories(ctx, &ecr.DescribeRepositoriesInput{
	})
	if err != nil {
		log.Fatal(err)
	}
	if len(repodata.Repositories) == 1 {
		fmt.Printf("Repository %s already exists skipping create", *repodata.Repositories[0].RepositoryName)
	}else{
		createEcr(ctx, &repoName, client)
	}
	return client
}

func createEcr(ctx context.Context, repoName *string, client *ecr.Client){
	output, err := client.CreateRepository(ctx, &ecr.CreateRepositoryInput{
		RepositoryName: repoName,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Repo %s created", *output.Repository.RepositoryName)
}

func pushImage(ctx context.Context, client *ecr.Client){
	manifest := `locationName:"dockerfile" min:"1" type:"string" required:"true"`
	imageTag := "latest"
	repoName := "skodaice"
	input := &ecr.PutImageInput{
		ImageManifest: &manifest,
		ImageTag: &imageTag,
		RepositoryName: &repoName,
	}
	output, err := client.PutImage(ctx, input)
	if err != nil {
		fmt.Println("Error pushing image manifest" + err.Error())
	}
	fmt.Println(output.Image.ImageId)
}

func createEcs(ctx context.Context){
	svcClient := getEcsClient()
	clusterName := "skodaiceecs"
	var clusterProviders = []string{"FARGATE"}
	output, err := svcClient.CreateCluster(ctx, &ecs.CreateClusterInput{
		CapacityProviders: clusterProviders,
		ClusterName: &clusterName,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(*output.Cluster.ClusterName)
}