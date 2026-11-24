import React, { useCallback, useEffect, useState } from 'react';
import {
  List,
  Button,
  Tag,
  Empty,
  Skeleton,
  message,
  Space,
  Typography,
  Modal,
  Form,
  Input,
  InputNumber,
  Switch,
  DatePicker,
  Table,
  Popconfirm,
  Flex,
  Tooltip,
} from 'antd';
import {
  CalendarOutlined,
  CheckCircleOutlined,
  PlusOutlined,
  FormOutlined,
  DeleteOutlined,
  EditOutlined,
  TeamOutlined,
  TrophyOutlined,
} from '@ant-design/icons';
import agent from '@/api/agent';
import type { Exam, ExamEnrollment, Course, CoursePoints, User } from '@/types';
import dayjs from 'dayjs';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

interface ExamsPanelProps {
  courseId: number;
  isStudent: boolean;
  canManage?: boolean;
  course?: Course | null;
  studentPoints?: CoursePoints[];
}

const STATUS_UNMARKED = 0;
const STATUS_FAILED = 1;
const STATUS_PASSED = 2;

const STANDARD_GRADES = [1.0, 1.3, 1.7, 2.0, 2.3, 2.7, 3.0, 3.3, 3.7, 4.0];

function calculateGrade(
  examPts: number,
  bonusPts: number,
  maxPts: number,
  bonusEnabled: boolean,
  maxBonusPercent: number
): { rawMark: string; finalMark: string; status: number } {
  const max = maxPts > 0 ? maxPts : 100;
  const passThreshold = 0.5 * max;

  if (examPts < passThreshold) {
    // Fail without bonus rule:
    return { rawMark: '5.0', finalMark: '5.0', status: STATUS_FAILED };
  }

  const denom = max - passThreshold;
  const rawFloat = denom > 0 ? 4.0 - ((examPts - passThreshold) / denom) * 3.0 : 1.0;
  const closestRaw = STANDARD_GRADES.reduce((prev, curr) =>
    Math.abs(curr - rawFloat) < Math.abs(prev - rawFloat) ? curr : prev
  );
  const rawMark = closestRaw.toFixed(1);

  if (!bonusEnabled || bonusPts <= 0) {
    return { rawMark, finalMark: rawMark, status: STATUS_PASSED };
  }

  // Cap bonus points to configured percentage of max exam points and add on the passing scale
  const maxBonusPts = max * ((maxBonusPercent > 0 ? maxBonusPercent : 10) / 100);
  const effectiveBonus = Math.min(bonusPts, maxBonusPts);
  const totalPts = Math.min(max, examPts + effectiveBonus);
  const finalFloat = denom > 0 ? Math.max(1.0, 4.0 - ((totalPts - passThreshold) / denom) * 3.0) : 1.0;
  const closestFinal = STANDARD_GRADES.reduce((prev, curr) =>
    Math.abs(curr - finalFloat) < Math.abs(prev - finalFloat) ? curr : prev
  );

  return { rawMark, finalMark: closestFinal.toFixed(1), status: STATUS_PASSED };
}

export const ExamsPanel: React.FC<ExamsPanelProps> = ({
  courseId,
  isStudent,
  canManage,
  course,
  studentPoints,
}) => {
  const { t } = useTranslation();
  const [exams, setExams] = useState<Exam[]>([]);
  const [enrollments, setEnrollments] = useState<ExamEnrollment[]>([]);
  const [loading, setLoading] = useState(true);
  const [busyExamId, setBusyExamId] = useState<number | null>(null);

  // Admin Exam Creation Modal
  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);
  const [createForm] = Form.useForm();
  const createBonusEnabled = Form.useWatch('bonus_enabled', createForm);

  // Admin Exam Edit Modal
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [editingExam, setEditingExam] = useState<Exam | null>(null);
  const [editLoading, setEditLoading] = useState(false);
  const [editForm] = Form.useForm();
  const editBonusEnabled = Form.useWatch('bonus_enabled', editForm);

  // Admin Grade / Examinees Modal
  const [examineeModalVisible, setExamineeModalVisible] = useState(false);
  const [activeExam, setActiveExam] = useState<Exam | null>(null);
  const [examinees, setExaminees] = useState<ExamEnrollment[]>([]);
  const [examineesLoading, setExamineesLoading] = useState(false);
  const [savingUserId, setSavingUserId] = useState<number | null>(null);
  const [usersMap, setUsersMap] = useState<Map<number, User>>(new Map());
  const [sheetBonusMap, setSheetBonusMap] = useState<Map<number, number>>(new Map());

  // Row edit state for grading modal: map user_id -> { exam_points, bonus_points, manual_mark }
  const [gradingDrafts, setGradingDrafts] = useState<
    Map<number, { exam_points: number; bonus_points: number; manual_mark: string }>
  >(new Map());

  const studentAchievedPoints = (studentPoints ?? []).reduce(
    (sum, p) => sum + (p.acquired_points || 0),
    0
  );
  const studentAchievablePoints = (studentPoints ?? []).reduce(
    (sum, p) => sum + (p.max_points || 0),
    0
  );
  const studentPercent =
    studentAchievablePoints > 0
      ? Math.round((studentAchievedPoints / studentAchievablePoints) * 100)
      : 0;
  const requiredPercent = course?.required_percentage ?? 0;
  const isEligibleToEnroll =
    !isStudent || (requiredPercent <= 0 || studentPercent >= requiredPercent);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const examsData = await agent.Exams.getAll(courseId);
      setExams(examsData);
      try {
        const enrollmentData = await agent.Account.getExamEnrollments();
        setEnrollments(enrollmentData.filter((e) => e.course_id === courseId));
      } catch {
        setEnrollments([]);
      }
    } catch {
      message.error(t('messages.updateFailed'));
    } finally {
      setLoading(false);
    }
  }, [courseId, t]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const enrollmentFor = (examId: number) => enrollments.find((e) => e.exam_id === examId);

  const handleEnroll = async (examId: number) => {
    if (!isEligibleToEnroll) {
      message.error(
        t('exam.eligibilityRequired', { required: requiredPercent, current: studentPercent })
      );
      return;
    }
    setBusyExamId(examId);
    try {
      await agent.Exams.enroll(courseId, examId);
      message.success(t('exam.enrollSuccess'));
      await fetchData();
    } catch (error: any) {
      message.error(error?.message || t('messages.updateFailed'));
    } finally {
      setBusyExamId(null);
    }
  };

  const handleDisenroll = async (exam: Exam) => {
    if (dayjs().isAfter(dayjs(exam.exam_time))) {
      message.error(t('exam.pastDeadlineUnenroll'));
      return;
    }
    setBusyExamId(exam.id);
    try {
      await agent.Exams.disenroll(courseId, exam.id);
      message.success(t('exam.disenrollSuccess'));
      await fetchData();
    } catch (error: any) {
      message.error(error?.message || t('exam.markedCannotLeave'));
    } finally {
      setBusyExamId(null);
    }
  };

  const handleCreateExam = async (values: any) => {
    setCreateLoading(true);
    try {
      await agent.Exams.create(courseId, {
        name: values.name,
        description: values.description,
        exam_time: values.exam_time.toISOString(),
        max_points: values.max_points ?? 100,
        bonus_enabled: values.bonus_enabled ?? false,
        bonus_percentage: values.bonus_enabled ? (values.bonus_percentage ?? 10) : 0,
      });
      message.success(t('exam.createSuccess'));
      setCreateModalVisible(false);
      createForm.resetFields();
      await fetchData();
    } catch (err: any) {
      message.error(err.message || t('exam.createFailed'));
    } finally {
      setCreateLoading(false);
    }
  };

  const handleOpenEdit = (exam: Exam) => {
    setEditingExam(exam);
    editForm.setFieldsValue({
      name: exam.name,
      description: exam.description,
      exam_time: dayjs(exam.exam_time),
      max_points: exam.max_points ?? 100,
      bonus_enabled: exam.bonus_enabled ?? false,
      bonus_percentage: exam.bonus_percentage ?? 10,
    });
    setEditModalVisible(true);
  };

  const handleUpdateExam = async (values: any) => {
    if (!editingExam) return;
    setEditLoading(true);
    try {
      await agent.Exams.update(courseId, editingExam.id, {
        name: values.name,
        description: values.description,
        exam_time: values.exam_time.toISOString(),
        max_points: values.max_points,
        bonus_enabled: values.bonus_enabled,
        bonus_percentage: values.bonus_enabled ? (values.bonus_percentage ?? 10) : 0,
      });
      message.success(t('exam.updateSuccess'));
      setEditModalVisible(false);
      await fetchData();
    } catch (err: any) {
      message.error(err.message || t('exam.updateFailed'));
    } finally {
      setEditLoading(false);
    }
  };

  const handleDeleteExam = async (examId: number) => {
    try {
      await agent.Exams.delete(courseId, examId);
      message.success(t('exam.deleteSuccess'));
      await fetchData();
    } catch {
      message.error(t('exam.deleteFailed'));
    }
  };

  const handleOpenExaminees = async (exam: Exam) => {
    setActiveExam(exam);
    setExamineeModalVisible(true);
    setExamineesLoading(true);
    try {
      const [data, courseEnrollments, summary] = await Promise.all([
        agent.Exams.getEnrollments(courseId, exam.id),
        agent.Courses.getEnrollments(courseId).catch(() => []),
        course?.bonus_mode === 1 ? agent.Grades.getSummary(courseId).catch(() => null) : null,
      ]);

      const uMap = new Map<number, User>();
      (courseEnrollments as any[]).forEach((e) => {
        if (e.user) uMap.set(e.user.id, e.user);
      });
      setUsersMap(uMap);

      const bMap = new Map<number, number>();
      if (summary?.achievements && exam.bonus_enabled) {
        const bonusPct = exam.bonus_percentage ?? 10;
        const maxBonus = Math.round((exam.max_points ?? 100) * (bonusPct / 100));
        const totalMaxCoursePoints =
          summary.total_max_points && summary.total_max_points > 0
            ? summary.total_max_points
            : (summary.sheets?.length ?? 0) * 10;
        summary.achievements.forEach((ach) => {
          const totalAcquired = ach.points.reduce((acc, p) => acc + (p > 0 ? p : 0), 0);
          const computedBonus =
            totalMaxCoursePoints > 0
              ? Math.min(maxBonus, Math.round((totalAcquired / totalMaxCoursePoints) * maxBonus))
              : 0;
          bMap.set(ach.user_info.id, computedBonus);
        });
      }
      setSheetBonusMap(bMap);

      setExaminees(data);
      const drafts = new Map<number, { exam_points: number; bonus_points: number; manual_mark: string }>();
      data.forEach((e) => {
        const defaultBonus: number =
          (e.bonus_points ?? 0) > 0
            ? (e.bonus_points ?? 0)
            : (course?.bonus_mode === 1 && exam.bonus_enabled ? (bMap.get(e.user_id) ?? 0) : 0);

        const calcInit = calculateGrade(
          e.exam_points ?? 0,
          defaultBonus,
          exam.max_points ?? 100,
          exam.bonus_enabled ?? false,
          exam.bonus_percentage ?? 10
        );
        const hasManualOverride = Boolean(
          e.mark && e.status !== STATUS_UNMARKED && e.mark !== calcInit.finalMark
        );

        drafts.set(e.user_id, {
          exam_points: e.exam_points ?? 0,
          bonus_points: defaultBonus,
          manual_mark: hasManualOverride ? (e.mark ?? '') : '',
        });
      });
      setGradingDrafts(drafts);
    } catch {
      message.error(t('exam.loadExamineesFailed'));
      setExaminees([]);
    } finally {
      setExamineesLoading(false);
    }
  };

  const handleSaveGrade = async (enrollment: ExamEnrollment) => {
    if (!activeExam) return;
    const draft = gradingDrafts.get(enrollment.user_id) ?? {
      exam_points: enrollment.exam_points ?? 0,
      bonus_points: enrollment.bonus_points ?? 0,
      manual_mark: '',
    };

    const calc = calculateGrade(
      draft.exam_points,
      draft.bonus_points,
      activeExam.max_points ?? 100,
      activeExam.bonus_enabled ?? false,
      activeExam.bonus_percentage ?? 10
    );

    const markToSave = draft.manual_mark || calc.finalMark;
    const parsedOverride = Number.parseFloat(draft.manual_mark);
    const statusToSave =
      draft.manual_mark && !Number.isNaN(parsedOverride)
        ? parsedOverride <= 4.0
          ? STATUS_PASSED
          : STATUS_FAILED
        : calc.status;

    setSavingUserId(enrollment.user_id);
    try {
      await agent.Exams.updateEnrollment(courseId, activeExam.id, {
        user_id: enrollment.user_id,
        status: statusToSave,
        mark: markToSave,
        exam_points: draft.exam_points,
        bonus_points: draft.bonus_points,
        raw_mark: calc.rawMark,
      });
      message.success(t('exam.gradeSaved'));
      setExaminees((prev) =>
        prev.map((e) =>
          e.user_id === enrollment.user_id
            ? {
                ...e,
                status: statusToSave,
                mark: markToSave,
                exam_points: draft.exam_points,
                bonus_points: draft.bonus_points,
                raw_mark: calc.rawMark,
              }
            : e
        )
      );
    } catch {
      message.error(t('exam.saveGradeFailed'));
    } finally {
      setSavingUserId(null);
    }
  };

  if (loading) return <Skeleton active paragraph={{ rows: 4 }} />;

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      {canManage && (
        <Flex justify="flex-end">
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => setCreateModalVisible(true)}
          >
            {t('exam.newExam')}
          </Button>
        </Flex>
      )}

      {exams.length === 0 ? (
        <Empty description={t('exam.noExams')} />
      ) : (
        <List
          dataSource={exams}
          renderItem={(exam) => {
            const enrollment = enrollmentFor(exam.id);
            const enrolled = Boolean(enrollment);
            const marked = enrollment ? enrollment.status !== STATUS_UNMARKED : false;
            const pastDeadline = dayjs().isAfter(dayjs(exam.exam_time));

            const actions: React.ReactNode[] = [];
            if (isStudent) {
              if (enrolled) {
                actions.push(
                  <Tooltip
                    title={
                      pastDeadline
                        ? t('exam.pastDeadlineUnenroll')
                        : marked
                        ? t('exam.markedCannotLeave')
                        : undefined
                    }
                    key="disenroll"
                  >
                    <Button
                      danger
                      loading={busyExamId === exam.id}
                      disabled={marked || pastDeadline}
                      onClick={() => handleDisenroll(exam)}
                    >
                      {t('exam.disenroll')}
                    </Button>
                  </Tooltip>
                );
              } else {
                actions.push(
                  <Tooltip
                    title={
                      !isEligibleToEnroll
                        ? t('exam.eligibilityRequired', { required: requiredPercent, current: studentPercent })
                        : pastDeadline
                        ? t('exam.pastDeadlineEnroll')
                        : undefined
                    }
                    key="enroll"
                  >
                    <Button
                      type="primary"
                      loading={busyExamId === exam.id}
                      disabled={!isEligibleToEnroll || pastDeadline}
                      onClick={() => handleEnroll(exam.id)}
                    >
                      {t('exam.enroll')}
                    </Button>
                  </Tooltip>
                );
              }
            }

            if (canManage) {
              actions.push(
                <Button
                  key="grade"
                  icon={<FormOutlined />}
                  type="primary"
                  ghost
                  onClick={() => handleOpenExaminees(exam)}
                >
                  {t('exam.gradeExaminees')}
                </Button>,
                <Button
                  key="edit"
                  icon={<EditOutlined />}
                  onClick={() => handleOpenEdit(exam)}
                />,
                <Popconfirm
                  key="delete"
                  title={t('exam.deleteConfirmTitle')}
                  description={t('exam.deleteConfirmDesc')}
                  onConfirm={() => handleDeleteExam(exam.id)}
                  okText={t('common.delete')}
                  cancelText={t('common.cancel')}
                >
                  <Button danger icon={<DeleteOutlined />} />
                </Popconfirm>
              );
            }

            return (
              <List.Item actions={actions}>
                <List.Item.Meta
                  title={
                    <Space wrap>
                      <span>{exam.name}</span>
                      {canManage && (
                        <Tag icon={<TeamOutlined />} color="cyan">
                          {exam.enrolled_count ?? 0} {t('exam.enrolled')}
                        </Tag>
                      )}
                      <Tag icon={<TrophyOutlined />} color="purple">
                        {t('exam.maxPointsBadge', { points: exam.max_points ?? 100 })}{' '}
                        {exam.bonus_enabled ? t('exam.bonusPctBadge', { pct: exam.bonus_percentage ?? 10 }) : ''}
                      </Tag>
                      {enrolled ? (
                        <Tag icon={<CheckCircleOutlined />} color="success">
                          {t('exam.enrolled')}
                        </Tag>
                      ) : (
                        isStudent && <Tag>{t('exam.notEnrolled')}</Tag>
                      )}
                      {marked && enrollment?.mark && (
                        <Tag color={enrollment.status === STATUS_PASSED ? 'green' : 'red'}>
                          {`${t('exam.mark')}: ${enrollment.mark}`}
                        </Tag>
                      )}
                    </Space>
                  }
                  description={
                    <Space direction="vertical" size={2} style={{ width: '100%' }}>
                      <Text type="secondary">
                        <CalendarOutlined /> {t('exam.examTime')}: {dayjs(exam.exam_time).format('MMM D, YYYY HH:mm')}
                      </Text>
                      <Typography.Paragraph ellipsis={{ rows: 2 }} style={{ marginBottom: 0 }}>
                        {exam.description}
                      </Typography.Paragraph>
                    </Space>
                  }
                />
              </List.Item>
            );
          }}
        />
      )}

      {/* Admin Create Exam Modal */}
      <Modal
        title={t('exam.newExam')}
        open={createModalVisible}
        onCancel={() => setCreateModalVisible(false)}
        onOk={() => createForm.submit()}
        confirmLoading={createLoading}
        okText={t('common.create')}
        cancelText={t('common.cancel')}
      >
        <Form form={createForm} layout="vertical" onFinish={handleCreateExam}>
          <Form.Item name="name" label={t('exam.formName')} rules={[{ required: true, message: t('exam.formNameReq') }]}>
            <Input placeholder={t('exam.formNamePlaceholder')} />
          </Form.Item>
          <Form.Item name="description" label={t('exam.formDesc')} rules={[{ required: true, message: t('exam.formDescReq') }]}>
            <Input.TextArea rows={3} placeholder={t('exam.formDescPlaceholder')} />
          </Form.Item>
          <Form.Item name="exam_time" label={t('exam.formDate')} rules={[{ required: true, message: t('exam.formDateReq') }]}>
            <DatePicker showTime style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="max_points" label={t('exam.formMaxPoints')} initialValue={100} rules={[{ required: true }]}>
            <InputNumber min={1} max={1000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="bonus_enabled" label={t('exam.formBonusEnabled')} valuePropName="checked" initialValue={false}>
            <Switch />
          </Form.Item>
          {createBonusEnabled && (
            <Form.Item name="bonus_percentage" label={t('exam.formBonusPct')} initialValue={10} rules={[{ required: true }]}>
              <InputNumber min={1} max={50} style={{ width: '100%' }} />
            </Form.Item>
          )}
        </Form>
      </Modal>

      {/* Admin Edit Exam Modal */}
      <Modal
        title={t('exam.editExam')}
        open={editModalVisible}
        onCancel={() => setEditModalVisible(false)}
        onOk={() => editForm.submit()}
        confirmLoading={editLoading}
        okText={t('common.save')}
        cancelText={t('common.cancel')}
      >
        <Form form={editForm} layout="vertical" onFinish={handleUpdateExam}>
          <Form.Item name="name" label={t('exam.formName')} rules={[{ required: true, message: t('exam.formNameReq') }]}>
            <Input />
          </Form.Item>
          <Form.Item name="description" label={t('exam.formDesc')} rules={[{ required: true, message: t('exam.formDescReq') }]}>
            <Input.TextArea rows={3} />
          </Form.Item>
          <Form.Item name="exam_time" label={t('exam.formDate')} rules={[{ required: true, message: t('exam.formDateReq') }]}>
            <DatePicker showTime style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="max_points" label={t('exam.formMaxPoints')} rules={[{ required: true }]}>
            <InputNumber min={1} max={1000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="bonus_enabled" label={t('exam.formBonusEnabled')} valuePropName="checked">
            <Switch />
          </Form.Item>
          {editBonusEnabled && (
            <Form.Item name="bonus_percentage" label={t('exam.formBonusPct')} initialValue={10} rules={[{ required: true }]}>
              <InputNumber min={1} max={50} style={{ width: '100%' }} />
            </Form.Item>
          )}
        </Form>
      </Modal>

      {/* Admin Examinee Grade Modal */}
      <Modal
        title={t('exam.examineesModalTitle', { name: activeExam?.name ?? '', max: activeExam?.max_points ?? 100 })}
        open={examineeModalVisible}
        onCancel={() => setExamineeModalVisible(false)}
        footer={null}
        width={920}
      >
        {examineesLoading ? (
          <Skeleton active paragraph={{ rows: 5 }} />
        ) : examinees.length === 0 ? (
          <Empty description={t('exam.noExaminees')} />
        ) : (
          <Table
            dataSource={examinees}
            rowKey={(r) => r.user_id}
            pagination={{ pageSize: 10 }}
            columns={[
              {
                title: t('roles.student'),
                dataIndex: 'user_id',
                key: 'user_id',
                width: 180,
                render: (uid: number) => {
                  const student = usersMap.get(uid);
                  if (!student) return `User #${uid}`;
                  return (
                    <div>
                      <div style={{ fontWeight: 600 }}>{student.first_name} {student.last_name}</div>
                      <div style={{ fontSize: 11, color: '#888' }}>
                        {student.student_number ? `Matr. ${student.student_number} • ` : ''}{student.email}
                      </div>
                    </div>
                  );
                },
              },
              {
                title: t('exam.examPtsCol', { max: activeExam?.max_points ?? 100 }),
                key: 'exam_points',
                width: 130,
                render: (_: unknown, record: ExamEnrollment) => {
                  const draft = gradingDrafts.get(record.user_id) ?? {
                    exam_points: record.exam_points ?? 0,
                    bonus_points: record.bonus_points ?? 0,
                    manual_mark: '',
                  };
                  return (
                    <InputNumber
                      min={0}
                      max={activeExam?.max_points ?? 100}
                      value={draft.exam_points}
                      onChange={(val) => {
                        const newPts = val ?? 0;
                        setGradingDrafts((prev) => {
                          const next = new Map(prev);
                          next.set(record.user_id, { ...draft, exam_points: newPts, manual_mark: '' });
                          return next;
                        });
                      }}
                    />
                  );
                },
              },
              {
                title:
                  !activeExam?.bonus_enabled || course?.bonus_mode === 0
                    ? t('exam.bonusPtsOff')
                    : course?.bonus_mode === 2
                    ? t('exam.bonusPtsManual')
                    : t('exam.bonusPtsAuto'),
                key: 'bonus_points',
                width: 140,
                render: (_: unknown, record: ExamEnrollment) => {
                  const draft = gradingDrafts.get(record.user_id) ?? {
                    exam_points: record.exam_points ?? 0,
                    bonus_points: record.bonus_points ?? 0,
                    manual_mark: '',
                  };
                  const maxBonusPts = activeExam?.bonus_enabled
                    ? Math.round((activeExam.max_points ?? 100) * ((activeExam.bonus_percentage ?? 10) / 100))
                    : 0;
                  const bonusDisabled = course?.bonus_mode !== 2 || !activeExam?.bonus_enabled;
                  const suggested = sheetBonusMap.get(record.user_id);
                  return (
                    <Space direction="vertical" size={2}>
                      <InputNumber
                        min={0}
                        max={maxBonusPts}
                        disabled={bonusDisabled}
                        value={draft.bonus_points}
                        onChange={(val) => {
                          const newBonus = val ?? 0;
                          setGradingDrafts((prev) => {
                            const next = new Map(prev);
                            next.set(record.user_id, { ...draft, bonus_points: newBonus, manual_mark: '' });
                            return next;
                          });
                        }}
                      />
                      {course?.bonus_mode === 1 && activeExam?.bonus_enabled && suggested !== undefined && suggested > 0 && draft.bonus_points !== suggested && (
                        <Button
                          size="small"
                          type="link"
                          style={{ padding: 0, fontSize: 11 }}
                          onClick={() => {
                            setGradingDrafts((prev) => {
                              const next = new Map(prev);
                              next.set(record.user_id, { ...draft, bonus_points: suggested, manual_mark: '' });
                              return next;
                            });
                          }}
                        >
                          {t('exam.useSheet', { pts: suggested })}
                        </Button>
                      )}
                    </Space>
                  );
                },
              },
              {
                title: t('exam.gradeCalcCol'),
                key: 'calc',
                width: 200,
                render: (_: unknown, record: ExamEnrollment) => {
                  const draft = gradingDrafts.get(record.user_id) ?? {
                    exam_points: record.exam_points ?? 0,
                    bonus_points: record.bonus_points ?? 0,
                    manual_mark: '',
                  };
                  const calc = calculateGrade(
                    draft.exam_points,
                    draft.bonus_points,
                    activeExam?.max_points ?? 100,
                    activeExam?.bonus_enabled ?? false,
                    activeExam?.bonus_percentage ?? 10
                  );
                  return (
                    <Space direction="vertical" size={2}>
                      <Space wrap>
                        <Tag color={calc.status === STATUS_PASSED ? 'green' : 'red'}>
                          {calc.status === STATUS_PASSED ? t('grades.passed') : t('grades.failed')}
                        </Tag>
                        <span>{t('exam.rawLabel')} <strong>{calc.rawMark}</strong></span>
                        {calc.status === STATUS_PASSED && draft.bonus_points > 0 && activeExam?.bonus_enabled && (
                          <Tag color="purple">{t('exam.finalLabel')} <strong>{calc.finalMark}</strong></Tag>
                        )}
                      </Space>
                    </Space>
                  );
                },
              },
              {
                title: t('exam.recordedMarkCol'),
                key: 'mark',
                width: 140,
                render: (_: unknown, record: ExamEnrollment) => {
                  const draft = gradingDrafts.get(record.user_id) ?? {
                    exam_points: record.exam_points ?? 0,
                    bonus_points: record.bonus_points ?? 0,
                    manual_mark: '',
                  };
                  const calc = calculateGrade(
                    draft.exam_points,
                    draft.bonus_points,
                    activeExam?.max_points ?? 100,
                    activeExam?.bonus_enabled ?? false,
                    activeExam?.bonus_percentage ?? 10
                  );
                  const displayMark = draft.manual_mark || calc.finalMark;
                  return (
                    <Input
                      value={displayMark}
                      placeholder={calc.finalMark}
                      onChange={(e) => {
                        const val = e.target.value;
                        setGradingDrafts((prev) => {
                          const next = new Map(prev);
                          next.set(record.user_id, { ...draft, manual_mark: val });
                          return next;
                        });
                      }}
                    />
                  );
                },
              },
              {
                title: t('common.actions'),
                key: 'action',
                width: 90,
                render: (_: unknown, record: ExamEnrollment) => (
                  <Button
                    size="small"
                    type="primary"
                    loading={savingUserId === record.user_id}
                    onClick={() => handleSaveGrade(record)}
                  >
                    {t('common.save')}
                  </Button>
                ),
              },
            ]}
          />
        )}
      </Modal>
    </Space>
  );
};

export default ExamsPanel;
