// ZLUseDevManageDlg.h : header file
//

#if !defined(AFX_ZLUSEDEVMANAGEDLG_H__C6D1C6E1_4A50_480F_B6A1_4532B1C48C72__INCLUDED_)
#define AFX_ZLUSEDEVMANAGEDLG_H__C6D1C6E1_4A50_480F_B6A1_4532B1C48C72__INCLUDED_

#if _MSC_VER > 1000
#pragma once
#endif // _MSC_VER > 1000

/////////////////////////////////////////////////////////////////////////////
// CZLUseDevManageDlg dialog
#define DEV_COL_STATUS	0
#define DEV_COL_NAME	1
#define DEV_COL_IP		2
#define DEV_COL_PORT	3
#define DEV_COL_ID		4
	
class CZLUseDevManageDlg : public CDialog
{
public:
// Construction
public:
	CZLUseDevManageDlg(CWnd* pParent = NULL);	// standard constructor
	~CZLUseDevManageDlg();
	int GetCurSel();

// Dialog Data
	//{{AFX_DATA(CZLUseDevManageDlg)
	enum { IDD = IDD_ZLUSEDEVMANAGE_DIALOG };
	CListCtrl	m_DevListCtrl;
	//}}AFX_DATA

	// ClassWizard generated virtual function overrides
	//{{AFX_VIRTUAL(CZLUseDevManageDlg)
	protected:
	virtual void DoDataExchange(CDataExchange* pDX);	// DDX/DDV support
	//}}AFX_VIRTUAL

// Implementation
protected:
	HICON m_hIcon;

	// Generated message map functions
	//{{AFX_MSG(CZLUseDevManageDlg)
	virtual BOOL OnInitDialog();
	afx_msg void OnSysCommand(UINT nID, LPARAM lParam);
	afx_msg void OnPaint();
	afx_msg HCURSOR OnQueryDragIcon();
	afx_msg void OnStartSearch();
	afx_msg void OnParamEdit();
	afx_msg void OnDblclkDevList(NMHDR* pNMHDR, LRESULT* pResult);
	afx_msg void OnManualAdd();
	afx_msg void OnAbout();
	afx_msg void OnP2pManage();
	afx_msg void OnSafeConnect();
	//}}AFX_MSG
	DECLARE_MESSAGE_MAP()
};

//{{AFX_INSERT_LOCATION}}
// Microsoft Visual C++ will insert additional declarations immediately before the previous line.

#endif // !defined(AFX_ZLUSEDEVMANAGEDLG_H__C6D1C6E1_4A50_480F_B6A1_4532B1C48C72__INCLUDED_)
