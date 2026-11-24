import React, { useEffect, useState } from 'react';
import {
  Card,
  Form,
  InputNumber,
  Input,
  Button,
  Descriptions,
  Tag,
  Spin,
  Divider,
  message,
  Checkbox,
  Flex,
  Space,
  Typography,
} from 'antd';
import { DownloadOutlined, TeamOutlined, UserOutlined } from '@ant-design/icons';
import { useParams, useNavigate } from 'react-router-dom';
import agent from '@/api/agent';
import type { Grade, GradeOverrideItem } from '@/types';
import { useTranslation } from 'react-i18next';
import dayjs from 'dayjs';

const { Paragraph } = Typography;

interface MemberOverrideState {
  enabled: boolean;
  points: number;
  feedback: string;
}

// SubmissionGrading lets a tutor review a single graded submission and record
// points + feedback.
//
// CRITICAL PATH: grade contract.
// A grade is addressed by its own id (grade.go Context keys off `grade_id`),
// NOT by the submission id. We therefore:
//   - load the grade via GET /courses/{course_id}/grades/{grade_id}
//     (returns GradeResponse: student, submission_id, file_url, test logs),
//   - download the submission file via
//     GET /courses/{course_id}/submissions/{submission_id}/file, and
//   - persist points/feedback via PUT /courses/{course_id}/grades/{grade_id}
//     with { acquired_points, feedback } (GradeRequest requires feedback).
const SubmissionGrading: React.FC = () => {
  const { courseId, gradeId } = useParams<{ courseId: string; gradeId: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [form] = Form.useForm();
  const [grade, setGrade] = useState<Grade | null>(null);
  const [memberOverrides, setMemberOverrides] = useState<Record<number, MemberOverrideState>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (courseId && gradeId) {
      fetchData();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [courseId, gradeId]);

  const fetchData = async () => {
    setLoading(true);
    try {
      const gradeData = await agent.Grades.get(parseInt(courseId!), parseInt(gradeId!));
      setGrade(gradeData);
      form.setFieldsValue({
        acquired_points: gradeData.acquired_points,
        feedback: gradeData.feedback,
      });

      const initialOverrides: Record<number, MemberOverrideState> = {};
      if (gradeData.teammate_grades) {
        for (const tm of gradeData.teammate_grades) {
          initialOverrides[tm.user_id] = {
            enabled: tm.is_individual_override,
            points: tm.acquired_points,
            feedback: tm.feedback,
          };
        }
      }
      setMemberOverrides(initialOverrides);
    } catch (error) {
      message.error(t('grades.loadFailed'));
    } finally {
      setLoading(false);
    }
  };

  const onFinish = async (values: { acquired_points: number; feedback: string }) => {
    if (!grade) return;
    setSaving(true);
    try {
      const overrides: GradeOverrideItem[] = [];
      const clearOverrideUserIDs: number[] = [];

      if (grade.teammate_grades) {
        for (const tm of grade.teammate_grades) {
          const state = memberOverrides[tm.user_id];
          if (state?.enabled) {
            overrides.push({
              user_id: tm.user_id,
              acquired_points: state.points,
              feedback: state.feedback,
            });
          } else if (tm.is_individual_override) {
            clearOverrideUserIDs.push(tm.user_id);
          }
        }
      }

      await agent.Grades.update(parseInt(courseId!), grade.id, {
        acquired_points: values.acquired_points,
        feedback: values.feedback,
        overrides: overrides.length > 0 ? overrides : undefined,
        clear_override_user_ids:
          clearOverrideUserIDs.length > 0 ? clearOverrideUserIDs : undefined,
      });
      message.success(t('grades.gradeSaved'));
      navigate(-1);
    } catch (error: any) {
      message.error(error?.message || t('messages.saveFailed'));
    } finally {
      setSaving(false);
    }
  };

  const handleDownload = async () => {
    if (!grade) return;
    try {
      await agent.Submissions.downloadFile(
        parseInt(courseId!),
        grade.submission_id,
        `submission-${grade.submission_id}.zip`,
      );
    } catch (error) {
      message.error(t('messages.downloadFailed'));
    }
  };

  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', padding: '48px 0' }}>
        <Spin size="large" />
      </div>
    );
  }

  if (!grade) {
    return <Card title={t('grades.gradeSubmission')}>{t('grades.notFound')}</Card>;
  }

  const studentName = grade.user
    ? `${grade.user.first_name} ${grade.user.last_name}`
    : `User #${grade.submission_id}`;

  return (
    <Card
      title={t('grades.gradeSubmission')}
      extra={
        grade.file_url ? (
          <Button icon={<DownloadOutlined />} onClick={handleDownload}>
            {t('grades.downloadSubmission')}
          </Button>
        ) : (
          <Tag>{t('grades.noFile')}</Tag>
        )
      }
    >
      <Descriptions column={1} bordered style={{ marginBottom: 24 }}>
        <Descriptions.Item label={t('grades.student')}>{studentName}</Descriptions.Item>
        {grade.team_id && (
          <Descriptions.Item label={t('team.myTeam')}>
            <Tag color="cyan">
              <TeamOutlined /> {t('team.teamNumber', { num: grade.team_number ?? `#${grade.team_id}` })}
            </Tag>
          </Descriptions.Item>
        )}
        {grade.user?.email && (
          <Descriptions.Item label={t('profile.email')}>{grade.user.email}</Descriptions.Item>
        )}
        <Descriptions.Item label={t('submission.publicTests')}>
          <Tag color={grade.public_test_status === 1 ? 'green' : 'red'}>
            {grade.public_test_status === 1 ? t('grades.passed') : t('grades.failed')}
          </Tag>
        </Descriptions.Item>
        {grade.updated_at && (
          <Descriptions.Item label={t('grades.lastUpdated')}>
            {dayjs(grade.updated_at).format('MMM D, YYYY HH:mm')}
          </Descriptions.Item>
        )}
      </Descriptions>

      {grade.public_test_log && (
        <>
          <Divider orientation="left">{t('submission.testLog')}</Divider>
          <pre
            style={{
              // Neutral translucent grey adapts to both light and dark themes,
              // matching SheetDetail's log styling.
              background: 'rgba(128,128,128,0.12)',
              padding: '12px',
              borderRadius: '4px',
              fontSize: '12px',
              overflow: 'auto',
              whiteSpace: 'pre-wrap',
            }}
          >
            {grade.public_test_log}
          </pre>
        </>
      )}

      <Divider orientation="left">{t('course.grading')}</Divider>
      <Form form={form} layout="vertical" onFinish={onFinish}>
        <Form.Item
          name="acquired_points"
          label={t('grades.points')}
          rules={[{ required: true, message: t('grades.pointsRequired') }]}
        >
          <InputNumber min={0} style={{ width: '100%' }} />
        </Form.Item>

        {/* The backend GradeRequest marks feedback as Required, so mirror that. */}
        <Form.Item
          name="feedback"
          label={t('grades.feedback')}
          rules={[{ required: true, message: t('grades.feedbackRequired') }]}
        >
          <Input.TextArea rows={6} />
        </Form.Item>

        {grade.teammate_grades && grade.teammate_grades.length > 1 && (
          <>
            <Divider orientation="left">
              <Space>
                <TeamOutlined />
                <span>{t('grades.teammateOverrides')}</span>
              </Space>
            </Divider>
            <Paragraph type="secondary">{t('grades.teammateOverridesHelp')}</Paragraph>
            <Space direction="vertical" style={{ width: '100%', marginBottom: 24 }} size="middle">
              {grade.teammate_grades.map((tm) => {
                const isOverride = memberOverrides[tm.user_id]?.enabled ?? false;
                return (
                  <Card
                    key={tm.user_id}
                    size="small"
                    title={
                      <Flex justify="space-between" align="center" wrap="wrap" gap="small">
                        <Space>
                          <UserOutlined />
                          <span>
                            {tm.first_name} {tm.last_name}
                          </span>
                          {isOverride ? (
                            <Tag color="purple">{t('grades.individualOverride')}</Tag>
                          ) : (
                            <Tag color="default">{t('grades.inheritsTeam')}</Tag>
                          )}
                        </Space>
                        <Checkbox
                          checked={isOverride}
                          onChange={(e) => {
                            const checked = e.target.checked;
                            setMemberOverrides((prev) => ({
                              ...prev,
                              [tm.user_id]: {
                                enabled: checked,
                                points:
                                  prev[tm.user_id]?.points ??
                                  form.getFieldValue('acquired_points') ??
                                  0,
                                feedback:
                                  prev[tm.user_id]?.feedback ??
                                  form.getFieldValue('feedback') ??
                                  '',
                              },
                            }));
                          }}
                        >
                          {t('grades.enableOverride')}
                        </Checkbox>
                      </Flex>
                    }
                  >
                    {isOverride && (
                      <Space direction="vertical" style={{ width: '100%', marginTop: 8 }}>
                        <div>
                          <label style={{ display: 'block', marginBottom: 4, fontWeight: 500 }}>
                            {t('grades.overridePoints')}
                          </label>
                          <InputNumber
                            min={0}
                            value={memberOverrides[tm.user_id]?.points}
                            onChange={(val) =>
                              setMemberOverrides((prev) => ({
                                ...prev,
                                [tm.user_id]: { ...prev[tm.user_id], points: val ?? 0 },
                              }))
                            }
                            style={{ width: '100%' }}
                          />
                        </div>
                        <div>
                          <label style={{ display: 'block', marginBottom: 4, fontWeight: 500 }}>
                            {t('grades.overrideFeedback')}
                          </label>
                          <Input.TextArea
                            rows={3}
                            value={memberOverrides[tm.user_id]?.feedback}
                            onChange={(e) =>
                              setMemberOverrides((prev) => ({
                                ...prev,
                                [tm.user_id]: { ...prev[tm.user_id], feedback: e.target.value },
                              }))
                            }
                          />
                        </div>
                      </Space>
                    )}
                  </Card>
                );
              })}
            </Space>
          </>
        )}

        <Form.Item>
          <Button type="primary" htmlType="submit" loading={saving}>
            {t('grades.saveGrade')}
          </Button>
          <Button style={{ marginLeft: 8 }} onClick={() => navigate(-1)}>
            {t('common.cancel')}
          </Button>
        </Form.Item>
      </Form>
    </Card>
  );
};

export default SubmissionGrading;
