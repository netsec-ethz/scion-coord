angular.module('scionApp')
    .controller('adminCtrl', ['$rootScope', '$scope', 'adminService', '$location', '$window', '$http',
        function ($rootScope, $scope, adminService, $location, $window, $http) {
            $scope.redirectIfNotAdmin = function () {
                if (!$rootScope.user["IsAdmin"]) {
                    $location.path('/user');
                }
            };

            $scope.adminPageData = function () {
                adminService.adminPageData().then(
                    function (data) {
                        console.log(data);
                        $rootScope.user = data["User"];
                        // option to allow the email template to be changed
                        // $scope.emailTemplate = data["EmailTemplate"];
                        $scope.organisation = $rootScope.user["Organisation"];
                        $scope.redirectIfNotAdmin();
                        $scope.defaultInvitation = function () {
                            return {
                                Organisation: $scope.organisation,
                                error: false
                            };
                        };
                        $scope.resetInvitations = function () {
                            $scope.invitations = [$scope.defaultInvitation()];
                        };
                        $scope.resetInvitations();
                    },
                    function (response) {
                        console.log(response);
                        if (response.status === 401 || response.status === 403) {
                            $location.path('/user');
                        }
                    });
            };

            $scope.adminPageData();
            $scope.error = "";
            $scope.message = "";

            $scope.dismissSuccess = function () {
                $scope.message = "";
            };

            $scope.dismissError = function () {
                $scope.error = "";
            };
        }
    ]);
