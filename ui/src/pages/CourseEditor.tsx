import React, { useEffect, useState } from 'react';
import { Form, Input, Button, Card, DatePicker, InputNumber, Switch, Divider, message, Select } from 'antd';
import { useNavigate, useParams } from 'react-router-dom';
import agent from '@/api/agent';
import dayjs from 'dayjs';
import { useTranslation } from 'react-i18next';

const CourseEditor: React.FC = () => {
  const { courseId } = useParams<{ courseId: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [initialLoading, setInitialLoading] = useState(!!courseId);
  const teamsEnabled = Form.useWatch('teams_enabled', form);

  useEffect(() => {
    if (courseId) {
      loadCourse();
    }
  }, [courseId]);

  const loadCourse = async () => {
    try {
      const course = await agent.Courses.get(parseInt(courseId!));
      form.setFieldsValue({
        ...course,
        begins_at: dayjs(course.begins_at),
        ends_at: dayjs(course.ends_at),
        teams_enabled: course.teams_enabled ?? false,
        team_formation_open: course.team_formation_open ?? false,
        max_team_size: course.max_team_size ?? 2,
        enforce_same_exercise_group: course.enforce_same_exercise_group ?? false,
        bonus_mode: course.bonus_mode ?? 1,
        group_solver: course.group_solver ?? 'optimal',
        min_group_preferences: course.min_group_preferences ?? 3,
      });
    } catch (error) {
      message.error(t('course.loadFailed'));
    } finally {
      setInitialLoading(false);
    }
  };

  const onFinish = async (values: any) => {
    setLoading(true);
    try {
      const data = {
        ...values,
        begins_at: values.begins_at.toISOString(),
        ends_at: values.ends_at.toISOString(),
      };

      if (courseId) {
        await agent.Courses.update(parseInt(courseId), data);
        message.success(t('course.updated'));
      } else {
        await agent.Courses.create(data);
        message.success(t('course.created'));
      }
      navigate('/courses');
    } catch (error) {
      message.error(courseId ? t('course.updateFailed') : t('course.createFailed'));
    } finally {
      setLoading(false);
    }
  };

  if (initialLoading) return <Card loading />;

  return (
    <Card title={courseId ? t('course.editCourse') : t('course.createCourse')}>
      <Form form={form} layout="vertical" onFinish={onFinish}>
        <Form.Item name="name" label={t('course.courseName')} rules={[{ required: true }]}>
          <Input />
        </Form.Item>

        <Form.Item name="description" label={t('course.description')} rules={[{ required: true }]}>
          <Input.TextArea rows={4} />
        </Form.Item>

        <Form.Item name="begins_at" label={t('course.startDate')} rules={[{ required: true }]}>
          <DatePicker style={{ width: '100%' }} />
        </Form.Item>

        <Form.Item name="ends_at" label={t('course.endDate')} rules={[{ required: true }]}>
          <DatePicker style={{ width: '100%' }} />
        </Form.Item>

        <Form.Item name="required_percentage" label={t('course.requiredPercentage')} rules={[{ required: true }]} initialValue={50}>
          <InputNumber min={0} max={100} style={{ width: '100%' }} />
        </Form.Item>

        <Form.Item name="bonus_mode" label={t('course.bonusCalculationMode')} initialValue={1} extra={t('course.bonusModeHelp')}>
          <Select
            options={[
              { value: 0, label: t('course.bonusModeDisabled') },
              { value: 1, label: t('course.bonusModeAuto') },
              { value: 2, label: t('course.bonusModeManual') },
            ]}
          />
        </Form.Item>

        <Divider orientation="left">{t('course.tutorialGroupSettings')}</Divider>

        <Form.Item
          name="group_solver"
          label={t('course.defaultAllocationSolver')}
          initialValue="optimal"
          extra={t('course.defaultAllocationSolverHelp')}
        >
          <Select
            options={[
              { value: 'optimal', label: t('course.solverOptimal') },
              { value: 'regret', label: t('course.solverRegret') },
              { value: 'greedy', label: t('course.solverGreedy') },
            ]}
          />
        </Form.Item>

        <Form.Item
          name="min_group_preferences"
          label={t('course.minPositivePreferences')}
          initialValue={3}
          rules={[{ required: true, type: 'number', min: 1, max: 20 }]}
          extra={t('course.minPositivePreferencesHelp')}
        >
          <InputNumber min={1} max={20} style={{ width: '100%' }} />
        </Form.Item>

        <Divider orientation="left">{t('team.teamSettings')}</Divider>

        <Form.Item
          name="teams_enabled"
          label={t('team.enableTeams')}
          valuePropName="checked"
          initialValue={false}
        >
          <Switch />
        </Form.Item>

        {teamsEnabled && (
          <>
            <Form.Item
              name="team_formation_open"
              label={t('team.teamFormationOpen')}
              valuePropName="checked"
              initialValue={true}
              extra={t('team.teamFormationOpenHelp')}
            >
              <Switch />
            </Form.Item>

            <Form.Item
              name="max_team_size"
              label={t('team.maxTeamSize')}
              rules={[{ required: true }]}
              initialValue={2}
            >
              <InputNumber min={1} max={50} style={{ width: '100%' }} />
            </Form.Item>

            <Form.Item
              name="enforce_same_exercise_group"
              label={t('team.enforceSameExerciseGroup')}
              valuePropName="checked"
              initialValue={false}
              extra={t('team.enforceSameExerciseGroupHelp')}
            >
              <Switch />
            </Form.Item>
          </>
        )}

        <Form.Item>
          <Button type="primary" htmlType="submit" loading={loading}>
            {courseId ? t('course.updateCourse') : t('course.createCourse')}
          </Button>
          <Button style={{ marginLeft: 8 }} onClick={() => navigate('/courses')}>
            {t('common.cancel')}
          </Button>
        </Form.Item>
      </Form>
    </Card>
  );
};

export default CourseEditor;
