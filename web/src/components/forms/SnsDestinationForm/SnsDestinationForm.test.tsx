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

import React from 'react';
import { render, fireEvent, waitFor, waitMs } from 'test-utils';
import { SeverityEnum, SnsConfig } from 'Generated/schema';
import SnsDestinationForm from './index';

const emptyInitialValues = {
  outputId: null,
  displayName: '',
  defaultForSeverity: [],
  outputConfig: {},
};

const validTopicArn = 'arn:aws:sns:us-east-2:123456789012:MyTopic';
const displayName = 'SNS';
const severity = SeverityEnum.Critical;

const initialValues = {
  outputId: '123',
  displayName,
  outputConfig: {
    sns: {
      topicArn: validTopicArn,
    } as SnsConfig,
  },
  defaultForSeverity: [severity],
};

describe('SnsDestinationForm', () => {
  it('renders the correct fields', () => {
    const { getByLabelText, getByText } = render(
      <SnsDestinationForm onSubmit={() => {}} initialValues={emptyInitialValues} />
    );
    const displayNameField = getByLabelText('* Display Name');
    const topicArnField = getByLabelText('Topic ARN');
    const submitButton = getByText('Add Destination');
    expect(displayNameField).toBeInTheDocument();
    expect(topicArnField).toBeInTheDocument();
    Object.values(SeverityEnum).forEach(sev => {
      expect(getByText(sev)).toBeInTheDocument();
    });

    expect(submitButton).toHaveAttribute('disabled');
  });

  it('has proper validation', async () => {
    const { getByLabelText, getByText } = render(
      <SnsDestinationForm onSubmit={() => {}} initialValues={emptyInitialValues} />
    );
    const displayNameField = getByLabelText('* Display Name');
    const topicArnField = getByLabelText('Topic ARN');
    const submitButton = getByText('Add Destination');
    const criticalSeverityCheckBox = document.getElementById(severity);
    expect(criticalSeverityCheckBox).not.toBeNull();
    expect(submitButton).toHaveAttribute('disabled');

    fireEvent.change(displayNameField, { target: { value: displayName } });
    fireEvent.click(criticalSeverityCheckBox);
    await waitMs(50);
    expect(submitButton).toHaveAttribute('disabled');
    fireEvent.change(topicArnField, { target: { value: 'notValidTopicArn' } });
    await waitMs(50);
    // Topic ARN has specific form
    expect(submitButton).toHaveAttribute('disabled');
    fireEvent.change(topicArnField, { target: { value: validTopicArn } });
    await waitMs(50);
    expect(submitButton).not.toHaveAttribute('disabled');
  });

  it('submit is triggering successfully', async () => {
    const submitMockFunc = jest.fn();
    const { getByLabelText, getByText } = render(
      <SnsDestinationForm onSubmit={submitMockFunc} initialValues={emptyInitialValues} />
    );
    const displayNameField = getByLabelText('* Display Name');
    const topicArnField = getByLabelText('Topic ARN');
    const submitButton = getByText('Add Destination');
    const criticalSeverityCheckBox = document.getElementById(severity);
    expect(criticalSeverityCheckBox).not.toBeNull();
    expect(submitButton).toHaveAttribute('disabled');

    fireEvent.change(displayNameField, { target: { value: displayName } });
    fireEvent.click(criticalSeverityCheckBox);
    fireEvent.change(topicArnField, { target: { value: validTopicArn } });
    await waitMs(50);
    expect(submitButton).not.toHaveAttribute('disabled');

    fireEvent.click(submitButton);
    await waitFor(() => expect(submitMockFunc).toHaveBeenCalledTimes(1));
    expect(submitMockFunc).toHaveBeenCalledWith({
      outputId: null,
      displayName,
      outputConfig: initialValues.outputConfig,
      defaultForSeverity: [severity],
    });
  });

  it('should edit SNS Destination successfully', async () => {
    const submitMockFunc = jest.fn();
    const { getByLabelText, getByText } = render(
      <SnsDestinationForm onSubmit={submitMockFunc} initialValues={initialValues} />
    );
    const displayNameField = getByLabelText('* Display Name');
    const submitButton = getByText('Update Destination');
    expect(displayNameField).toHaveValue(initialValues.displayName);
    expect(submitButton).toHaveAttribute('disabled');

    const newDisplayName = 'New SNS Name';
    fireEvent.change(displayNameField, { target: { value: newDisplayName } });
    await waitMs(50);
    expect(submitButton).not.toHaveAttribute('disabled');

    fireEvent.click(submitButton);
    await waitFor(() => expect(submitMockFunc).toHaveBeenCalledTimes(1));
    expect(submitMockFunc).toHaveBeenCalledWith({
      outputId: initialValues.outputId,
      displayName: newDisplayName,
      outputConfig: initialValues.outputConfig,
      defaultForSeverity: initialValues.defaultForSeverity,
    });
  });
});
