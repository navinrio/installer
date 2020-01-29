package aws

import (
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/efs"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/terraform-providers/terraform-provider-aws/aws/internal/keyvaluetags"
)

func dataSourceAwsEfsFileSystem() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceAwsEfsFileSystemRead,

		Schema: map[string]*schema.Schema{
			"arn": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"creation_token": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringLenBetween(0, 64),
			},
			"encrypted": {
				Type:     schema.TypeBool,
				Computed: true,
			},
			"file_system_id": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},
			"kms_key_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"performance_mode": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"dns_name": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"tags": tagsSchemaComputed(),
		},
	}
}

func dataSourceAwsEfsFileSystemRead(d *schema.ResourceData, meta interface{}) error {
	efsconn := meta.(*AWSClient).efsconn

	describeEfsOpts := &efs.DescribeFileSystemsInput{}

	if v, ok := d.GetOk("creation_token"); ok {
		describeEfsOpts.CreationToken = aws.String(v.(string))
	}

	if v, ok := d.GetOk("file_system_id"); ok {
		describeEfsOpts.FileSystemId = aws.String(v.(string))
	}

	log.Printf("[DEBUG] Reading EFS File System: %s", describeEfsOpts)
	describeResp, err := efsconn.DescribeFileSystems(describeEfsOpts)
	if err != nil {
		return fmt.Errorf("Error retrieving EFS: %s", err)
	}
	if len(describeResp.FileSystems) != 1 {
		return fmt.Errorf("Search returned %d results, please revise so only one is returned", len(describeResp.FileSystems))
	}

	d.SetId(*describeResp.FileSystems[0].FileSystemId)

	tags, err := keyvaluetags.EfsListTags(efsconn, d.Id())

	if err != nil {
		return fmt.Errorf("error listing tags for EFS file system (%s): %s", d.Id(), err)
	}

	if err := d.Set("tags", tags.IgnoreAws().Map()); err != nil {
		return fmt.Errorf("error settings tags: %s", err)
	}

	var fs *efs.FileSystemDescription
	for _, f := range describeResp.FileSystems {
		if d.Id() == *f.FileSystemId {
			fs = f
			break
		}
	}
	if fs == nil {
		log.Printf("[WARN] EFS (%s) not found, removing from state", d.Id())
		d.SetId("")
		return nil
	}

	d.Set("creation_token", fs.CreationToken)
	d.Set("performance_mode", fs.PerformanceMode)

	fsARN := arn.ARN{
		AccountID: meta.(*AWSClient).accountid,
		Partition: meta.(*AWSClient).partition,
		Region:    meta.(*AWSClient).region,
		Resource:  fmt.Sprintf("file-system/%s", aws.StringValue(fs.FileSystemId)),
		Service:   "elasticfilesystem",
	}.String()

	d.Set("arn", fsARN)
	d.Set("file_system_id", fs.FileSystemId)
	d.Set("encrypted", fs.Encrypted)
	d.Set("kms_key_id", fs.KmsKeyId)

	region := meta.(*AWSClient).region
	err = d.Set("dns_name", resourceAwsEfsDnsName(*fs.FileSystemId, region))
	return err
}