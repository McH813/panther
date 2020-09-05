package awsglue

/**
 * Panther is a Cloud-Native SIEM for the Modern Security Team.
 * Copyright (C) 2020 Panther Labs Inc
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/glue"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/pkg/errors"
)

// Use this to tag the time partitioning used in a GlueTableMetadata table
type GlueTableTimebin int

const (
	GlueTableMonthly GlueTableTimebin = iota + 1
	GlueTableDaily
	GlueTableHourly
)

func (tb GlueTableTimebin) Validate() (err error) {
	switch tb {
	case GlueTableHourly, GlueTableDaily, GlueTableMonthly:
		return
	default:
		err = fmt.Errorf("unknown GlueTableMetadata table time bin: %d", tb)
	}
	return
}

func (tb GlueTableTimebin) Truncate(t time.Time) time.Time {
	switch tb {
	case GlueTableHourly:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
	case GlueTableDaily:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	case GlueTableMonthly:
		return time.Date(t.Year(), t.Month(), 0, 0, 0, 0, 0, t.Location())
	default:
		panic(fmt.Sprintf("unknown GlueTableMetadata table time bin: %d", tb))
	}
}

// Next returns the next time interval
func (tb GlueTableTimebin) Next(t time.Time) (next time.Time) {
	switch tb {
	case GlueTableHourly:
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, t.Location())
	case GlueTableDaily:
		return time.Date(t.Year(), t.Month(), t.Day()+1, t.Hour(), 0, 0, 0, t.Location())
	case GlueTableMonthly:
		return time.Date(t.Year(), t.Month()+1, t.Day(), t.Hour(), 0, 0, 0, t.Location())
	default:
		panic(fmt.Sprintf("unknown GlueTableMetadata table time bin: %d", tb))
	}
}

// PartitionsBefore returns an expression to scan for partitions before tm
func (tb GlueTableTimebin) PartitionsBefore(tm time.Time) string {
	tm = tm.UTC().Truncate(time.Hour)
	var values []string
	values = append(values, fmt.Sprintf("(year < %d)", tm.Year()))
	if tb >= GlueTableMonthly {
		values = append(values, fmt.Sprintf("(year = %d AND month < %02d)", tm.Year(), tm.Month()))
	}
	if tb >= GlueTableDaily {
		values = append(values, fmt.Sprintf("(year = %d AND month = %02d AND day < %02d)", tm.Year(), tm.Month(), tm.Day()))
	}
	if tb >= GlueTableHourly {
		values = append(values, fmt.Sprintf("(year = %d AND month = %02d AND day = %02d AND hour < %02d)", tm.Year(), tm.Month(), tm.Day(), tm.Hour()))
	}
	return strings.Join(values, " OR ")
}

// PartitionValuesFromTime returns an []*string values (used for Glue APIs)
func (tb GlueTableTimebin) PartitionValuesFromTime(t time.Time) (values []*string) {
	values = []*string{aws.String(fmt.Sprintf("%d", t.Year()))}

	if tb >= GlueTableMonthly {
		values = append(values, aws.String(fmt.Sprintf("%02d", t.Month())))
	}
	if tb >= GlueTableDaily {
		values = append(values, aws.String(fmt.Sprintf("%02d", t.Day())))
	}
	if tb >= GlueTableHourly {
		values = append(values, aws.String(fmt.Sprintf("%02d", t.Hour())))
	}
	return
}

func (tb GlueTableTimebin) PartitionTimeFromValues(values []string) (tm time.Time, ok bool) {
	if len(values) == 0 {
		return
	}
	year, err := strconv.Atoi(values[0])
	if err != nil {
		return
	}
	var month, day, hour int
	if len(values) > 1 && tb >= GlueTableMonthly {
		if month, err = strconv.Atoi(values[1]); err != nil {
			return
		}
	}
	if len(values) > 2 && tb >= GlueTableDaily {
		if day, err = strconv.Atoi(values[2]); err != nil {
			return
		}
	}
	if len(values) > 3 && tb >= GlueTableHourly {
		if hour, err = strconv.Atoi(values[3]); err != nil {
			return
		}
	}
	return time.Date(year, time.Month(month), day, hour, 0, 0, 0, time.UTC), true
}

// PartitionS3PathFromTime constructs the S3 path for this partition
func (tb GlueTableTimebin) PartitionS3PathFromTime(t time.Time) (s3Path string) {
	switch tb {
	case GlueTableHourly:
		return fmt.Sprintf("year=%d/month=%02d/day=%02d/hour=%02d/", t.Year(), t.Month(), t.Day(), t.Hour())
	case GlueTableDaily:
		return fmt.Sprintf("year=%d/month=%02d/day=%02d/", t.Year(), t.Month(), t.Day())
	default:
		return fmt.Sprintf("year=%d/month=%02d/", t.Year(), t.Month())
	}
}

// PartitionHasData checks if there is at least 1 s3 object in the partition
func (tb GlueTableTimebin) PartitionHasData(client s3iface.S3API, t time.Time, tableOutput *glue.GetTableOutput) (bool, error) {
	bucket, prefix, err := ParseS3URL(*tableOutput.Table.StorageDescriptor.Location)
	if err != nil {
		return false, errors.Wrapf(err, "Cannot parse s3 path: %s",
			*tableOutput.Table.StorageDescriptor.Location)
	}

	// list files w/pagination
	inputParams := &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucket),
		Prefix:  aws.String(prefix + tb.PartitionS3PathFromTime(t)),
		MaxKeys: aws.Int64(1), // look for at least 1
	}
	var hasData bool
	err = client.ListObjectsV2Pages(inputParams, func(page *s3.ListObjectsV2Output, isLast bool) bool {
		for _, value := range page.Contents {
			if *value.Size > 0 { // we only care about objects with size
				hasData = true
			}
		}
		return !hasData // "To stop iterating, return false from the fn function."
	})

	return hasData, err
}
