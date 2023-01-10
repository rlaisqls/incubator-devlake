/*
Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tasks

import (
	"github.com/apache/incubator-devlake/errors"
	"reflect"
	"strconv"
	"time"

	"github.com/apache/incubator-devlake/models/domainlayer/didgen"

	"github.com/apache/incubator-devlake/plugins/core/dal"

	"github.com/apache/incubator-devlake/models/domainlayer"
	"github.com/apache/incubator-devlake/models/domainlayer/ticket"
	"github.com/apache/incubator-devlake/plugins/core"
	"github.com/apache/incubator-devlake/plugins/helper"
	"github.com/apache/incubator-devlake/plugins/tapd/models"
)

func ConvertStory(taskCtx core.SubTaskContext) errors.Error {
	rawDataSubTaskArgs, data := CreateRawDataSubTaskArgs(taskCtx, RAW_STORY_TABLE, false)
	logger := taskCtx.GetLogger()
	db := taskCtx.GetDal()
	logger.Info("convert board:%d", data.Options.WorkspaceId)

	clauses := []dal.Clause{
		dal.From(&models.TapdStory{}),
		dal.Where("connection_id = ? AND workspace_id = ?", data.Options.ConnectionId, data.Options.WorkspaceId),
	}

	cursor, err := db.Cursor(clauses...)
	if err != nil {
		return err
	}
	defer cursor.Close()
	storyIdGen := didgen.NewDomainIdGenerator(&models.TapdStory{})
	converter, err := helper.NewDataConverter(helper.DataConverterArgs{
		RawDataSubTaskArgs: *rawDataSubTaskArgs,
		InputRowType:       reflect.TypeOf(models.TapdStory{}),
		Input:              cursor,
		Convert: func(inputRow interface{}) ([]interface{}, errors.Error) {
			toolL := inputRow.(*models.TapdStory)
			domainL := &ticket.Issue{
				DomainEntity: domainlayer.DomainEntity{
					Id: storyIdGen.Generate(toolL.ConnectionId, toolL.Id),
				},
				Url:                  toolL.Url,
				IssueKey:             strconv.FormatUint(toolL.Id, 10),
				Title:                toolL.Name,
				Type:                 toolL.StdType,
				Status:               toolL.StdStatus,
				StoryPoint:           int64(toolL.Size),
				OriginalStatus:       toolL.Status,
				ResolutionDate:       (*time.Time)(toolL.Completed),
				CreatedDate:          (*time.Time)(toolL.Created),
				UpdatedDate:          (*time.Time)(toolL.Modified),
				ParentIssueId:        storyIdGen.Generate(toolL.ConnectionId, toolL.ParentId),
				Priority:             toolL.Priority,
				TimeRemainingMinutes: int64(toolL.Remain),
				CreatorId:            getAccountIdGen().Generate(data.Options.ConnectionId, toolL.Creator),
				CreatorName:          toolL.Creator,
				AssigneeName:         toolL.Owner,
				Severity:             "",
				Component:            toolL.Feature,
			}
			if domainL.AssigneeName != "" {
				domainL.AssigneeId = getAccountIdGen().Generate(data.Options.ConnectionId, toolL.Owner)
			}
			if domainL.ResolutionDate != nil && domainL.CreatedDate != nil {
				domainL.LeadTimeMinutes = int64(domainL.ResolutionDate.Sub(*domainL.CreatedDate).Minutes())
			}
			results := make([]interface{}, 0, 2)
			boardIssue := &ticket.BoardIssue{
				BoardId: getWorkspaceIdGen().Generate(toolL.ConnectionId, toolL.WorkspaceId),
				IssueId: domainL.Id,
			}
			sprintIssue := &ticket.SprintIssue{
				SprintId: getIterIdGen().Generate(data.Options.ConnectionId, toolL.IterationId),
				IssueId:  domainL.Id,
			}
			results = append(results, domainL, boardIssue, sprintIssue)
			return results, nil
		},
	})
	if err != nil {
		return err
	}

	return converter.Execute()
}

var ConvertStoryMeta = core.SubTaskMeta{
	Name:             "convertStory",
	EntryPoint:       ConvertStory,
	EnabledByDefault: true,
	Description:      "convert Tapd story",
	DomainTypes:      []string{core.DOMAIN_TYPE_TICKET},
}
